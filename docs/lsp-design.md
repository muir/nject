# Design: an LSP for nject

Status: draft for discussion — no code yet.

## 1. What we want

nject resolves dependency injection chains at runtime using reflection. Type
mismatches are caught when `Bind()`/`Run()` executes, not when code is
written. The goal is to move that feedback into the editor:

1. **Completion** — while writing a provider (typically the parameter list of
   a function inside a chain), show which types are available at that point in
   the chain, i.e. everything provided upstream: outputs of earlier injectors,
   literal values, and the `inner()` arguments of enclosing wrappers. Like
   gopls showing available identifiers, but for the injection type-space.

2. **Diagnostics** — if a provider consumes a type that nothing upstream
   provides (for the chains it participates in), underline that parameter with
   an error, with the same message `Bind()` would eventually produce
   ("has no match for its input parameter X"). Also surface other bind-time
   errors: `MustConsume` violations, unused-but-`Required` conflicts,
   shadowing errors, etc.

3. **Go to provider** — "go to definition" on an input parameter's type jumps
   to the provider that actually supplies that value — following nject's real
   resolution logic (chain ordering, interface best-match from `match.go`,
   `Loose[T]`, reordering), not just the Go type declaration (gopls already
   does that part).

## 2. The core problem: static vs. runtime resolution

Everything nject knows, it learns from `reflect` at runtime:

- `characterize.go` classifies each provider (injector, static injector,
  wrapper, fallible, literal, final) from its `reflect.Type`.
- `include.go` solves which providers to include given desiredness
  (`Required`, `Desired`, `Shun`), consumption rules, and type demand.
- `match.go` picks the best concrete type for an interface parameter
  (layer number, package path, method count).
- `flows.go` computes per-provider input/output flows, including the special
  handling of wrapper `inner()` functions and `TerminalError`.

An LSP cannot execute the user's program, so it needs a **static shadow of
this algorithm** operating on `go/types` instead of `reflect`. That's the
heart of the project; the LSP protocol plumbing around it is comparatively
routine.

This decomposes into two separable analyses:

- **(A) Chain reconstruction** — from source code, figure out what the
  ordered provider list of each `Bind`/`Run` call is. This is where static
  analysis is inherently incomplete (chains can be built dynamically).
- **(B) Type-flow solving** — given a reconstructed chain, replicate
  characterize/include/match to compute: which providers are included, which
  provider satisfies each input, and what errors `Bind()` would report. This
  part can be made near-exact.

## 3. Architecture overview

```
┌────────────┐   stdio/LSP    ┌─────────────────────────────────────┐
│  Editor    │◄──────────────►│  nject-lsp (server binary)          │
│ (VS Code,  │                │                                     │
│  Neovim…)  │◄──────────────►│  ┌──────────┐  ┌──────────────────┐ │
└────────────┘   gopls (as    │  │ snapshot │  │ feature handlers │ │
                 usual, in    │  │ manager  │  │ diag/defn/compl/ │ │
                 parallel)    │  │(pkgs+    │  │ hover/codelens   │ │
                              │  │ overlays)│  └────────┬─────────┘ │
                              │  └────┬─────┘           │           │
                              │       ▼                 ▼           │
                              │  ┌───────────────────────────────┐  │
                              │  │  static analysis core         │  │
                              │  │  1. chain discovery           │  │
                              │  │  2. collection evaluator      │  │
                              │  │  3. type-flow solver          │  │
                              │  │     (characterize/include/    │  │
                              │  │      match over go/types)     │  │
                              │  └───────────────────────────────┘  │
                              └─────────────────────────────────────┘
                                       ▲
                                       │ same core, no LSP
                              ┌────────┴────────┐
                              │ njectcheck      │  go vet -vettool /
                              │ (go/analysis)   │  CI linting
                              └─────────────────┘
```

Key decisions:

- **Standalone server, not a gopls fork/plugin.** gopls has no stable plugin
  mechanism. Editors handle multiple language servers on the same file well:
  VS Code merges completion lists, shows multiple definition results as a
  peek list, and namespaces diagnostics by `source`. nject-lsp answers only
  nject-specific questions and stays silent otherwise, so it composes with
  gopls instead of competing with it.
- **The analysis core is a plain Go library**, independent of LSP types, so
  the same engine powers a `go/analysis` analyzer (`njectcheck`) usable with
  `go vet -vettool=` in CI. This also gives us a fast way to validate the
  core before any editor work happens (see §8 Phasing).
- **Separate Go module** (either a `github.com/muir/nject-lsp` repo or an
  `lsp/` subdirectory with its own `go.mod`). The nject library itself must
  not grow dependencies on `go/packages`, LSP protocol libs, etc.

Likely dependencies: `golang.org/x/tools/go/packages`,
`golang.org/x/tools/go/analysis`, `golang.org/x/tools/go/types/typeutil`,
and a maintained LSP protocol + jsonrpc2 library (`go.lsp.dev/protocol` or
vendored equivalents).

## 4. The static analysis core

### 4.1 Chain discovery

Scan the loaded packages for **roots**: calls to `nject.Run`, `nject.MustRun`,
`Collection.Bind`, `Collection.MustBind`, and the related callback/curry
entry points (`SetCallback`, `MustSetCallback`, `Curry`…). Identification is
by `types.Object` identity against the `github.com/muir/nject/v2` package —
robust against renamed imports and wrapper libraries can be added later
(npoint, nchi, ntest expose their own entry points; see §9).

A root gives us:

- the ordered argument list (the chain tail), and
- for `Bind(&invokeFunc, &initFunc)`, the invoke/init function pointer types,
  which define what enters the chain from the top (invoke inputs) and what
  must come back up (invoke outputs) — mirroring `bind.go`'s
  `characterizeInitInvoke`.

`Sequence`/`Cluster` calls found along the way become reusable **collection
values** referenced from roots.

### 4.2 Collection evaluator (the incomplete part)

To turn root arguments into an ordered provider list we need a small symbolic
evaluator over the SSA-ish structure of the code. Supported forms, in order
of how commonly they appear in real nject code:

| Form | Example | Support |
|---|---|---|
| direct literal args | `Run("x", func(...) {...}, 7)` | v1 |
| annotation wrappers | `Provide("name", fn)`, `Cacheable(fn)`, `Required(fn)`, `Memoize(fn)`, `Loose[T](fn)`, `NonFinal(fn)`, `MustConsume[T](fn)`, `Shun`, `Desired`, `AllowReturnShadowing[T]`, `OverridesError`, chained combinations | v1 |
| named sequences, package-level or local, single assignment | `var Common = nject.Sequence("common", ...)` used in many roots | v1 |
| functions returning `*Collection` (same module) | `func middleware() *Collection { return Sequence(...) }` | v1: inline one call level; deeper via fixed budget |
| cross-package sequences (same module + deps with source) | exported `*Collection` vars | v1 (packages are loaded with full syntax) |
| conditional / loop-built chains | `if debug { seq = seq.Append(...) }`, building `[]any` in a loop | **not resolved** → chain marked *opaque* |
| `Reflective` / `ReflectiveWrapper` / `MustMakeStructBuilder` / `GenerateFromInjectionChain` | runtime-constructed providers | **not resolved** → provider marked *opaque* |

**Opaqueness policy (important for trust):** a chain containing anything we
cannot resolve is analyzed in *degraded mode*:

- an opaque **provider** is treated as "may provide anything, may consume
  anything" — it suppresses *missing-provider* diagnostics downstream of it,
  but completion and go-to-provider still work for what we *do* know;
- an opaque **chain** (can't even establish ordering) gets no diagnostics at
  all, and a subtle code-lens/hover note: "chain not statically analyzable".

False positives are fatal for adoption; false negatives are fine. We only
report an error when we are certain nject would too.

### 4.3 Type-flow solver

A port of nject's own pipeline onto `go/types`:

1. **Characterize** each provider from its `types.Signature`: literal vs
   func; wrapper (first parameter is a func — the `inner()`), final (last in
   chain, after `NonFinal` shuffling), fallible (returns `TerminalError`),
   static vs run-time (per `Cacheable`/`MustCache`/`Memoize` and position
   relative to the invoke boundary — same predicate table as
   `characterize.go`).
2. **Flows**: per provider, inputs/outputs going down and returns going up,
   replicating `flows.go` (`effectiveOutputs`: a wrapper's downward outputs
   are its `inner()`'s parameters; `TerminalError` stripped from outputs and
   routed upward; final func returns flow up through wrappers).
3. **Reorder** when `Reorder` is in play (port of `reorder.go`'s topological
   ordering).
4. **Include/exclude solving**: port of `include.go` — providers included
   only if needed to satisfy the final func and `Required` providers,
   honoring `Shun`/`Desired`, `MustConsume`, `ConsumptionOptional`.
5. **Interface matching**: port of `match.go`'s `bestMatch` with identical
   scoring — highest layer, same package path, method count, tie-break —
   using `types.Implements` in place of `reflect.Type.Implements`, and
   honoring `Loose[T]` (assignability rather than exact match).

`reflect.Type` identity maps cleanly onto `go/types` identity via
`types.Identical` / `typeutil.Map`, with one caveat: named-type identity
across build configurations is stable within a single `go/packages` snapshot,
which is all we need.

**Solver output** (per root): for every provider —
included/excluded + why; for every input parameter — the resolved providing
provider(s) and source positions; for every output — its consumers; plus the
list of bind errors with positions. This one result object backs *all* LSP
features, so it's computed once per root per snapshot and cached.

### 4.4 Keeping the solver faithful: parity testing

The biggest long-term risk is semantic drift between the static solver and
nject itself. Mitigation: a **golden parity harness**. The nject repo already
has a large corpus of chains in `*_test.go` (including `matrix_test.go` which
enumerates provider-kind combinations). The harness:

1. runs each corpus chain through real nject with `Debugging` /
   `DetailedError` to capture the runtime truth (included set, per-input
   provider, error text);
2. runs the same source through the static solver;
3. diffs.

This runs in CI for both repos (an nject release gate can run the harness to
catch changes that would break the LSP).

A more radical alternative — refactoring nject's core so
characterize/include/match are generic over an abstract type representation
(`reflect.Type` at runtime, `go/types.Type` statically) — would eliminate
drift by construction. `type_codes.go` already funnels every type through a
single `typeCode` indirection, so it's feasible, but it is an invasive change
to a stable library and drags `go/types` near nject's dependency graph.
**Recommendation: start with the port + parity harness; revisit sharing the
core only if drift proves painful in practice.**

## 5. LSP feature mapping

### 5.1 Diagnostics (`textDocument/publishDiagnostics`)

- Missing provider: squiggle on the *parameter declaration* (or on the type
  expression) of the consuming function:
  `nject: no provider for "MyType" in chain "example" (Run at server.go:41)`.
- A `Sequence` used by several roots may fail in some and pass in others →
  one diagnostic per failing root, grouped via `relatedInformation` pointing
  at the root call.
- Other bind errors (`MustConsume` unconsumed, unresolvable reorder cycles,
  return-shadowing violations) attach to the offending provider expression.
- `source: "nject"`, so users can filter them, and gopls diagnostics remain
  untouched.
- Warning-level extras (opt-in): provider excluded from all chains
  ("dead provider"), a diagnostic nject itself never gives but the solver
  gets for free.

### 5.2 Completion (`textDocument/completion`)

Context: cursor inside the **parameter list of a function expression that the
evaluator attributes to a chain** (func literal passed to
`Run`/`Sequence`/`Provide`/…, or the declaration of a named function used in
a chain). Offer one item per type available at that chain position:

- label: the type (`*sql.DB`, `myapp.RequestID`)
- detail: `provided by NewDB (db.go:14)` — or `inner() arg of TxWrapper`
- inserts the fully qualified, import-aware type text (edits include the
  `additionalTextEdits` for the import, same mechanism gopls uses)

Ranking: closest layer first (mirrors `bestMatch` layer preference).
These items are additive to gopls's; the editor merges the lists. Since a
gopls type-name completion and ours may collide, ours use a distinct
`CompletionItem.labelDetails` and a sort prefix so they cluster.

Secondary context (later): inside the function *body*, no special handling —
gopls already completes parameters. Inside a `Sequence(...)` arg list,
completion of known `*Collection` variables in scope is a nice-to-have.

### 5.3 Go to provider (`textDocument/definition`)

On an input parameter (name or type) of a chain function, return
`LocationLink[]` to the **providing value**, per root:

- injector output → the return type expression of that injector (target
  range: the specific result field; if it's a func literal, its position)
- literal in the chain → the literal expression
- wrapper `inner()` argument → that parameter of the `inner` func type
- invoke-function input (for `Bind`) → the invoke func-pointer's parameter

Multiple roots resolving differently → multiple links; the editor shows a
picker. gopls will simultaneously return the type declaration; both appear,
which matches the user's mental model ("the type" vs "the value").

For symmetry, `textDocument/references` on an injector's *result* type lists
downstream consumers, and `implementation` is left to gopls.

### 5.4 Hover, code lens, inlay hints

- **Hover** on a chain function: `included in 3/3 chains · static injector ·
  provides *sql.DB → consumed by BeginTx, RunQuery`; on an excluded provider:
  the exclusion reason from the include solver.
- **Code lens** on each root: `nject: chain OK (12 providers, 9 included)` or
  `nject: 2 errors`; command opens a rendered chain view (the `String()`
  debug table as a virtual document).
- **Inlay hints** (opt-in): grey-out marker for excluded providers.

## 6. Server internals

- **Snapshot manager**: `go/packages` load of the workspace with
  `packages.NeedSyntax|NeedTypes|NeedTypesInfo|NeedDeps`, plus LSP overlay
  contents for unsaved buffers (`packages.Config.Overlay`). On change:
  debounce (~200 ms), reload only affected packages, re-run chain discovery
  on packages whose syntax changed, and re-solve only roots whose transitive
  inputs changed (chain → collection → provider dependency graph is tracked
  from the previous solve).
- **Incremental cost model**: chain discovery is a syntax walk (cheap);
  the solver is per-root and each root is typically < 100 providers, so
  solving is microseconds — the dominant cost is type-checking, same as any
  Go tool. Memory: one snapshot, shared `typeutil.Map` interners.
- While a file is mid-edit and doesn't parse, keep the last good solve for
  that package (stale-but-useful), like gopls does.

## 7. Editor integration

- **VS Code**: a thin extension (`nject-vscode`) that launches `nject-lsp`
  for `go` files. It declines all capabilities except the ones above, so
  gopls remains the primary server. Ship the server binary via
  `go install github.com/muir/nject-lsp/cmd/nject-lsp@latest` with the
  extension auto-installing/updating it (the pattern used by many Go tools).
- **Neovim / Helix / anything with LSP config**: document a snippet
  registering `nject-lsp` alongside gopls for Go filetypes. No special
  support needed — this is exactly why we stay a standard LSP server.
- **CI / no editor**: `njectcheck` via `go vet -vettool=$(which njectcheck)`
  or standalone, emitting the same diagnostics.

## 8. Phasing

1. **Static core + `njectcheck`** — chain discovery, evaluator for the v1
   forms, type-flow solver, parity harness against the nject test corpus.
   Deliverable: a CLI that prints bind errors for a module without running
   it. This is the risk-retiring milestone: if the solver can't be made
   faithful, we learn it here, before any LSP investment.
2. **LSP server: diagnostics + go-to-provider + hover** — snapshot manager,
   overlays, incremental re-solve; the three features that share the solve
   result most directly.
3. **Completion + code lens + VS Code extension packaging.**
4. **Breadth**: wrapper-library entry points (npoint/nchi/ntest roots),
   `Reflective` partial modeling, `Cluster` semantics refinements, deeper
   evaluator (interprocedural budget, `append`-built chains with constant
   shapes).

## 9. Open questions

1. **Repo placement** — separate repo (`muir/nject-lsp`) vs. nested module
   (`nject/lsp/`). Separate repo keeps nject's dependency surface pristine
   and lets the LSP version independently; nested module keeps the parity
   harness close to the corpus it consumes. Leaning: separate repo, with the
   parity harness vendoring nject's testdata by module dependency.
2. **Downstream frameworks** — much real-world nject usage is via ntest /
   npoint / nchi, where the root call lives in the framework. Do we hardcode
   knowledge of their entry points, or define a small annotation convention
   (e.g. a magic doc comment `//nject:chain-root` on functions whose variadic
   `any` params are chain tails)? The annotation scales better; frameworks
   could adopt it.
3. **Named non-literal providers** — when a chain references `SomeFunc` (an
   identifier, not a literal), diagnostics about its unsatisfied inputs could
   attach either at the reference in the chain or at `SomeFunc`'s declaration.
   Proposal: at the reference (the chain is what's wrong), with
   `relatedInformation` at the declaration.
4. **Generics** — providers that are instantiations of generic functions
   type-check fine under `go/types`, but chains built inside generic
   functions with type-parameter-dependent provider types may be unsolvable
   until instantiated. Proposal: solve per known instantiation if any exist
   in the workspace; otherwise opaque.
