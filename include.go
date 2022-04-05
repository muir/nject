package nject

import (
	"fmt"
)

type includeWorkingData struct {
	usesDetail      map[flowType]map[typeCode][]*provider
	uses            []*provider
	usesError       map[flowType]map[typeCode]error
	usedBy          []*provider
	usedByDetail    map[flowType]map[typeCode][]*provider
	mustConsumeFlow map[flowType]bool
	excluded        error
	clusterMembers  []*provider
	wantedInCluster bool
}

//
// Figuring out which providers to use is done in multiple steps:
//
// Combine MustConsume and ConsumptionOptional into the
// per flow directions.  fm.d.mustConsumeFlow set in
// computeDependenciesAndInclusion().  This introduces some
// asymetry between the downward chain and the returns chain.
//
// Compute the potential data flows between the providers.
// providesReturns().  This sets most of the includeWorkingData
// fields.
//
// Assume that all providers will be included in the chain.
//
// Validate the chain.  This clears fm.include and
// fm.cannotInclude and then sets them.  It returns error
// if the chain is invalid.
//
// Prune the chain of providers that are obviously not
// needed.  eliminateUnused()
//
// Generate a list of providers that maybe could be removed
// from the chain.  proposeEliminations().  This adds more
// asymmetry between the down chain and the returns chain.
//
// Iterate over the list of proposed elimnations, re-validating
// the chain with each one removed.  If the chain remains valid,
// that provider is excluded.  If the chain becomes invalid, the
// provider is restored to the chain.  This is an expensive
// process.  Call to validate the chain is somewhat larger than
// O(n) so this entire process is somewhat worse than O(n^2).
//
// When we're done with the proposed eliminations, we have our
// final set of providers.  We then re-do some earlier steps:
// calculate data flows, provideReturns(), and prune obvious
// deadweight, eliminateUnused().
//
// In addition to everything in includeWorkingData, the
// following fields of provider are set here:
//
// 	fm.downRmap
// 	fm.upRmap
// 	fm.cannotInclude
// 	fm.include
//	fm.whyIncluded
//	fm.wanted
//

func computeDependenciesAndInclusion(funcs []*provider, initF *provider) error {
	debugln("initial set of functions")
	doingReorder := rememberOriginalOrder(funcs)
	clusterLeaders := make(map[int32]*provider)
	for _, fm := range funcs {
		debugf("\t%s", fm)
		if fm.cluster != 0 {
			if leader, ok := clusterLeaders[fm.cluster]; ok {
				leader.d.clusterMembers = append(leader.d.clusterMembers, fm)
				fm.d.clusterMembers = nil
			} else {
				clusterLeaders[fm.cluster] = fm
				fm.d.clusterMembers = []*provider{fm}
			}
		}
		fm.d.mustConsumeFlow = make(map[flowType]bool)
		if fm.mustConsume {
			fm.d.mustConsumeFlow[outputParams] = true
		}
		if !fm.consumptionOptional {
			fm.d.mustConsumeFlow[returnParams] = true
		}
		if fm.required {
			fm.whyIncluded = "required"
		} else if fm.desired {
			fm.whyIncluded = "desired"
		} else if fm.flows[outputParams] != nil && len(fm.flows[outputParams]) == 0 {
			fm.whyIncluded = "auto-desired (injector with no outputs)"
			if fm.cluster != 0 {
				fm.d.wantedInCluster = true
			}
			fm.wanted = true
		}
	}
	debugln("calculate flows, initial")
	err := providesReturns(funcs, initF)
	if err != nil {
		return err
	}

	debugln("check chain validity, no provider excluded, except failed reorders")
	err = validateChainMarkIncludeExclude(doingReorder, funcs, true)
	if err != nil {
		return err
	}

	for _, fm := range funcs {
		if fm.cannotInclude != nil {
			debugf("Excluding %s: %s", fm, fm.cannotInclude)
			fm.d.excluded = fm.cannotInclude
			fm.include = false
		}
	}

	eliminateUnused(funcs)

	tryWithout := func(without ...*provider) bool {
		if len(without) == 1 {
			if without[0].wanted && without[0].d.wantedInCluster {
				// working around a bug: don't try to eliminate single
				// wanted functions from clusters
				return false
			}
			debugf("check chain validity, excluding %s", without[0])
		} else {
			debugf("check chain validity, excluding %d in cluster %s", len(without), without[0])
		}
		for _, fm := range without {
			fm.d.excluded = fmt.Errorf("excluded to see what happens")
			if len(without) > 1 {
				if fm.d.wantedInCluster {
					fm.wanted = false
				}
			}
		}
		debugf("lenth of without before: %d", len(without))
		// nolint:govet
		err := validateChainMarkIncludeExclude(doingReorder, funcs, false)
		debugf("lenth of without after: %d", len(without))
		for _, fm := range without {
			if err == nil {
				fm.d.excluded = fmt.Errorf("not required, not desired, not necessary")
			} else {
				fm.whyIncluded = fmt.Sprintf("if excluded then: %s", err)
				fm.d.excluded = nil
			}
			if len(without) > 1 {
				if fm.d.wantedInCluster {
					fm.wanted = true
				}
			}
		}
		return err == nil
	}

	// Attempt to eliminate providers
	postCheck := make([]*provider, 0, len(funcs))
	for _, fm := range proposeEliminations(funcs) {
		if fm.d.excluded != nil {
			continue
		}
		if fm.d.clusterMembers != nil {
			if tryWithout(fm.d.clusterMembers...) {
				continue
			}
		}
		tryWithout(fm)
	}

	eliminateUnused(postCheck)

	debugln("final set of functions")
	for _, fm := range funcs {
		if fm.d.excluded == nil {
			fm.cannotInclude = nil
			debugf("\tinclude %s --- %s", fm, fm.whyIncluded)
		} else {
			if fm.cannotInclude == nil {
				fm.cannotInclude = fm.d.excluded
			}
			debugf("\texclude %s --- %s", fm, fm.cannotInclude)
		}
	}

	debugln("final calculate flows")
	err = providesReturns(funcs, initF)
	if err != nil {
		return fmt.Errorf("internal error: uh oh")
	}
	debugf("final check chain validity")
	err = validateChainMarkIncludeExclude(doingReorder, funcs, true)
	if err != nil {
		return fmt.Errorf("internal error: uh oh #2")
	}

	return nil
}

func validateChainMarkIncludeExclude(doingReorder bool, funcs []*provider, canRemoveDesired bool) error {
	if doingReorder {
		restoreOriginalOrder(funcs)
	}
	remainingFuncs := make([]*provider, 0, len(funcs))
	for _, fm := range funcs {
		if fm.d.excluded == nil {
			fm.include = true
			fm.cannotInclude = nil
			remainingFuncs = append(remainingFuncs, fm)
		} else {
			if fm.required {
				return fmt.Errorf("is required and excluded")
			}
			fm.cannotInclude = fm.d.excluded
			fm.include = false
		}
	}
	if doingReorder {
		remainingFuncs = reorder(remainingFuncs)
	}
	return checkFlows(remainingFuncs, len(funcs), canRemoveDesired)
}

func checkFlows(funcs []*provider, numFuncs int, canRemoveDesired bool) error {
	todo := funcs
	redo := make([]*provider, 0, len(funcs)*6)
	for len(todo) > 0 {
		seen := make([]bool, numFuncs)
		debugf("\tstarting check pass with %d providers", len(todo))
	Todo:
		for _, fm := range todo {
			if seen[fm.chainPosition] {
				debugf("\talready done: %s", fm)
				continue
			}
			seen[fm.chainPosition] = true
			if fm.cannotInclude != nil {
				if fm.required {
					debugf("\tchain invalid required but: %s: %s", fm, fm.cannotInclude)
					return fm.errorf("required but %s", fm.cannotInclude)
				}
				if (fm.wanted || fm.desired) && !canRemoveDesired && fm.d.excluded == nil {
					debugf("\tchain invalid wanted but: %s: %s", fm, fm.cannotInclude)
					return fm.errorf("wanted but %s", fm.cannotInclude)
				}
				if fm.include {
					debugf("\tprovider now excluded: %s: %s", fm, fm.cannotInclude)
					fm.include = false
					redo = append(redo, fm.d.usedBy...)
				} else {
					debugf("\tprovider already excluded: %s: %s", fm, fm.cannotInclude)
				}
				continue
			}

			debugf("\tchecking %s", fm)

			// This checks for inputs with no provider
			for param, errors := range fm.d.usesError {
				for tc, err := range errors {
					fm.cannotInclude = err
					redo = append(redo, fm)
					debugf("\t\trequire error on %s %s: %s", param, tc, err)
					continue Todo
				}
			}

			// This checks providers of inputs
			for param, sources := range fm.d.usesDetail {
			Source:
				for tc, plist := range sources {
					var extra string
					for _, p := range plist {
						if p.include {
							debugf("\t\t\tfound source for %s %s: %s", param, tc, p)
							continue Source
						}
						debugf("\t\t\tcannot provide %s %s: %s: %s", param, tc, p, p.cannotInclude)
						extra = fmt.Sprintf(" (not provided by %s because %s)", p, p.cannotInclude)
					}
					fm.cannotInclude = fmt.Errorf("no provider for %s in %s%s", tc, param, extra)
					redo = append(redo, fm)
					debugf("\t\tno source %s %s  %s: %s", param, tc, fm, fm.cannotInclude)
					continue Todo
				}
			}

			// This checks for mustConsume violations
			for param, tclist := range fm.flows {
				if !fm.d.mustConsumeFlow[param] {
					continue
				}
			Param:
				for _, tc := range tclist {
					var extra string
					for _, p := range fm.d.usedByDetail[param][tc] {
						if p.include {
							debugf("\t\t\tfound consumer of %s %s: %s", param, tc, p)
							continue Param
						}
						debugf("\t\t\tcannot consume %s %s: %s: %s", param, tc, p, p.cannotInclude)
						extra = fmt.Sprintf(" (not consumed by %s because %s)", p, p.cannotInclude)
					}
					fm.cannotInclude = fmt.Errorf("no consumer for %s in %s%s", tc, param, extra)
					redo = append(redo, fm)
					debugf("\t\tnot consumed %s %s %s: %s", param, tc, fm, fm.cannotInclude)
					continue Todo
				}
			}
			debugf("\t\tprovider still valid: %s", fm)
		}
		todo = redo
		redo = make([]*provider, 0, len(redo)*2)
	}
	debugln("\tchain is valid")
	return nil
}

func providesReturns(funcs []*provider, initF *provider) error {
	debugln("calculating provides/returns")
	for _, fm := range funcs {
		fm.d.usedByDetail = make(map[flowType]map[typeCode][]*provider)
		fm.d.usesDetail = make(map[flowType]map[typeCode][]*provider)
		fm.d.uses = nil
		fm.d.usesError = make(map[flowType]map[typeCode]error)
		fm.d.usedBy = nil
	}
	provide := make(interfaceMap)
	for i, fm := range funcs {
		if fm.cannotInclude != nil {
			debugf("\tskipping on downard path %s: %s", fm, fm.cannotInclude)
			continue
		}
		if fm.class == invokeFunc && initF != nil {
			initF.bypassRmap = make(map[typeCode]typeCode)
			err := requireParameters(initF, provide, bypassParams, outputParams, initF.bypassRmap, "returned value")
			if err != nil {
				return err
			}
		}
		err := requireParameters(fm, provide, inputParams, outputParams, fm.downRmap, "input")
		if err != nil {
			return err
		}
		provideParameters(fm, provide, outputParams, inputParams, i+2)
	}

	// Upwards chain
	returns := make(interfaceMap)
	for i := len(funcs) - 1; i >= 0; i-- {
		fm := funcs[i]
		if fm.cannotInclude != nil {
			debugf("\tskipping on upward path %s: %s", fm, fm.cannotInclude)
			continue
		}
		err := requireParameters(fm, returns, returnedParams, returnParams, fm.upRmap, "expected return")
		if err != nil {
			return err
		}
		provideParameters(fm, returns, returnParams, returnedParams, len(funcs)-i+2)
	}
	return nil
}

func provideParameters(
	fm *provider,
	available interfaceMap,
	param flowType,
	inParam flowType,
	position int,
) {
	debugf("\tproviding %s for %s", param, fm)
	incoming := make(map[typeCode]bool)
	for _, in := range fm.flows[inParam] {
		incoming[in] = true
	}
	fm.d.usedByDetail[param] = make(map[typeCode][]*provider)
	for _, out := range fm.flows[param] {
		if out == noTypeCode {
			debugln("\t\tskipping no-type")
			continue
		}
		debugf("\t\tproviding %s from %s", out, fm)
		available.Add(out, position, fm)
	}
}

func requireParameters(
	fm *provider,
	available interfaceMap,
	param flowType,
	outParam flowType,
	rMap map[typeCode]typeCode,
	purpose string,
) error {
	debugf("\trequire %s for %s", purpose, fm)
	fm.d.usesError[param] = make(map[typeCode]error)
	fm.d.usesDetail[param] = make(map[typeCode][]*provider)
	for _, in := range fm.flows[param] {
		if in == noTypeCode {
			debugf("\t\tskipping %s: not a real type", in)
			continue
		}
		found, dependsOn, err := available.bestMatch(in, purpose)
		if err != nil {
			debugf("\t\tcannot find %s %s: %s", param, in, err)
			fm.d.usesError[param][in] = err
			continue
		}
		if len(dependsOn) == 0 {
			return fmt.Errorf("internal error: dependsOn should not be empty for %s %s in %s", param, in, fm)
		}
		rMap[in] = found
		for _, dep := range dependsOn {
			debugf("\t\tadding dependency for %s: uses %s", in, dep)
			fm.d.usesDetail[param][in] = append(fm.d.usesDetail[param][in], dep)
			fm.d.uses = append(fm.d.uses, dep)

			debugf("\t\tadding used-by %s %s: %s", outParam, in, dep)
			dep.d.usedBy = append(dep.d.usedBy, fm)
			dep.d.usedByDetail[outParam][in] = append(dep.d.usedByDetail[outParam][in], fm)
			if dep.d.mustConsumeFlow[outParam] {
				fm.d.usedBy = append(fm.d.usedBy, dep)
			}
		}
	}
	return nil
}

func eliminateUnused(check []*provider) {
	debugln("eliminate those that no longer have any consumers")
PostCheck:
	for len(check) > 0 {
		var fm *provider
		fm, check = check[0], check[1:]
		if fm.required || fm.desired || fm.wanted || !fm.include || fm.d.excluded != nil {
			continue
		}
		for _, dep := range fm.d.usedBy {
			if dep.include {
				debugf("\t%s included by %s", fm, dep)
				continue PostCheck
			}
		}
		fm.include = false
		fm.cannotInclude = fmt.Errorf("not used by any remaining providers")
		fm.d.excluded = fm.cannotInclude
		debugf("\tno included users for: %s", fm)
		check = append(check, fm.d.uses...)
	}
}

func proposeEliminations(funcs []*provider) []*provider {
	debugln("pick providers that should be considered for exclusion")
	kept := make([]bool, len(funcs))
	for _, fg := range []struct {
		direction  string
		flowGroups []flowType
		useLast    bool
	}{
		{"down", []flowType{inputParams, bypassParams}, true},
		{"up", []flowType{returnedParams}, false},
	} {
		keep := make([]bool, len(funcs))
		toKeep := make([]*provider, 0, len(funcs))
		for _, fm := range funcs {
			if fm.d.excluded != nil {
				continue
			}
			if fm.required || fm.desired || (fm.wanted && !fm.d.wantedInCluster) {
				toKeep = append(toKeep, fm)
			}
		}
		for len(toKeep) > 0 {
			var fm *provider
			fm, toKeep = toKeep[0], toKeep[1:]
			if keep[fm.chainPosition] {
				debugf("\talready kept: %s", fm)
				continue
			}
			debugf("\tkeeping %s %s", fg.direction, fm)
			keep[fm.chainPosition] = true
			kept[fm.chainPosition] = true
			for _, param := range fg.flowGroups {
				for tc, users := range fm.d.usesDetail[param] {
					debugf("\t\tsourcing %s %s", param, tc)
					deps := make([]*provider, 0, len(users))
					for _, dep := range users {
						if dep.cannotInclude == nil && dep.d.excluded == nil {
							debugf("\t\t\tcan get it from %s", dep)
							deps = append(deps, dep)
						}
					}
					if len(deps) > 0 {
						var k *provider
						if fg.useLast {
							k = deps[len(deps)-1]
						} else {
							k = deps[0]
						}
						if !keep[k.chainPosition] {
							debugf("\t\t\tfor %s %s, keeping %s", param, tc, k)
							toKeep = append(toKeep, k)
							if k.whyIncluded == "" {
								k.whyIncluded = fmt.Sprintf("used by %s (%s)", fm, fm.whyIncluded)
							}
						} else {
							debugf("\t\t\tfor %s %s, no need to keep %s", param, tc, k)
						}
					}
				}
			}
		}
	}
	proposal := make([]*provider, 0, len(funcs))
	for i, fm := range funcs {
		if !kept[i] || fm.shun {
			proposal = append(proposal, fm)
		}
	}
	return proposal
}
