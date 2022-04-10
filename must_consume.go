package nject

import (
	"reflect"
	"strings"
)

// generateCheckers has side effects.  It populates:
//	fm.d.provides
//	fm.d.requires
//	fm.d.returns
//	fm.d.recevies
//	fm.d.hasProvide
//	fm.d.hasRequire
//	fm.d.hasReturns
//	fm.d.hasReceives
//	fm.d.transitiveRequire
//	fm.d.transitiveDesire
func generateCheckers(funcs []*provider) func(funcs []*provider, canRemoveDesired bool) error {
	providersIndex := make(map[reflect.Type][]int)
	returnsIndex := make(map[reflect.Type][]int)
	requiresIndex := make(map[reflect.Type][]int)
	receviesIndex := make(map[reflect.Type][]int)
	providersFuncs := make(map[reflect.Type][]*provider)
	returnsFuncs := make(map[reflect.Type][]*provider)
	requiresFuncs := make(map[reflect.Type][]*provider)
	receviesFuncs := make(map[reflect.Type][]*provider)
	for _, fm := range funcs {
		fm.d.receives, fm.d.returns = fm.UpFlows()
		fm.d.hasReceives, fm.d.hasReturns = has(fm.d.receives), has(fm.d.returns)
		for i, t := range fm.d.returns {
			returnsIndex[t] = append(returnsIndex[t], i)
			returnsFuncs[t] = append(returnsFuncs[t], fm)
		}
		for i, t := range fm.d.receives {
			receviesIndex[t] = append(receviesIndex[t], i)
			receviesFuncs[t] = append(receviesFuncs[t], fm)
		}
		fm.d.requires, fm.d.provides = fm.DownFlows()
		fm.d.hasRequire, fm.d.hasProvide = has(fm.d.requires), has(fm.d.provides)
		for i, t := range fm.d.provides {
			providersIndex[t] = append(providersIndex[t], i)
			providersFuncs[t] = append(providersFuncs[t], fm)
		}
		for i, t := range fm.d.requires {
			requiresIndex[t] = append(requiresIndex[t], i)
			requiresFuncs[t] = append(requiresFuncs[t], fm)
		}
	}
	deps := make(map[int][]int) // if key is required then list is required
	for i, fm := range funcs {
		for _, t := range fm.d.receives {
			if len(returnsIndex[t]) == 1 {
				deps[i] = append(deps[i], returnsIndex[t][0])
			}
		}
		if !fm.consumptionOptional {
			for _, t := range fm.d.returns {
				if len(receviesIndex[t]) == 1 {
					deps[i] = append(deps[i], receviesIndex[t][0])
				}
			}
		}
		if fm.mustConsume {
			for _, t := range fm.d.provides {
				if len(requiresIndex[t]) == 1 {
					deps[i] = append(deps[i], requiresIndex[t][0])
				}
			}
		}
		for _, t := range fm.d.requires {
			if len(providersIndex[t]) == 1 {
				deps[i] = append(deps[i], providersIndex[t][0])
			}
		}
	}

	seen := make([]bool, len(funcs))
	todo := make([]int, 0, len(funcs))
	for i, fm := range funcs {
		if fm.required || fm.desired {
			todo = append(todo, i)
			seen[i] = true
			if fm.required {
				fm.d.transitiveRequire = fm.errorf("required")
			} else {
				fm.d.transitiveDesire = fm.errorf("desired")
			}
		}
	}
	for len(todo) > 0 {
		i := todo[0]
		todo = todo[1:]
		if seen[i] {
			continue
		}
		seen[i] = true
		source := funcs[i]
		for _, d := range deps[i] {
			fm := funcs[d]
			if source.d.transitiveRequire != nil {
				if fm.d.transitiveRequire == nil {
					fm.d.transitiveRequire = fm.errorf("required to satisfy %s", funcs[i].d.transitiveRequire)
				}
			} else {
				if fm.d.transitiveDesire == nil && fm.d.transitiveRequire == nil {
					fm.d.transitiveDesire = fm.errorf("required to satisfy %s", funcs[i].d.transitiveDesire)
				}
			}
		}
		todo = append(todo, deps[i]...)
	}

	mustConsume := make([]*provider, 0, len(funcs))
	required := make([]*provider, 0, len(funcs))
	desired := make([]*provider, 0, len(funcs))
	for _, fm := range funcs {
		if fm.d.transitiveRequire != nil {
			required = append(required, fm)
		} else if fm.d.transitiveDesire != nil {
			desired = append(desired, fm)
		}
		if fm.mustConsume {
			mustConsume = append(mustConsume, fm)
		}
	}
	return func(funcs []*provider, canRemoveDesired bool) error {
		for _, fm := range required {
			if !fm.include || fm.d.excluded != nil || fm.cannotInclude != nil {
				return fm.d.transitiveRequire
			}
		}
		if !canRemoveDesired {
			for _, fm := range desired {
				if !fm.include || fm.d.excluded != nil || fm.cannotInclude != nil {
					return fm.d.transitiveDesire
				}
			}
		}
		if len(mustConsume) > 0 {
			for i, fm := range funcs {
				fm.d.ultimatePosition = i
			}
		MustConsumer:
			for _, fm := range mustConsume {
				if fm.include && fm.d.excluded == nil && fm.cannotInclude == nil {
					for _, t := range fm.d.provides {
						explain := make([]string, 0, len(requiresFuncs[t]))
						for _, rfm := range requiresFuncs[t] {
							if rfm.d.excluded != nil || !rfm.include {
								explain = append(explain, fm.errorf("not included").Error())
								continue
							}
							if rfm.cannotInclude != nil {
								explain = append(explain, rfm.cannotInclude.Error())
								continue
							}
							if rfm.d.ultimatePosition < fm.d.ultimatePosition {
								explain = append(explain, rfm.errorf("in the wrong order").Error())
								continue
							}
							continue MustConsumer
						}
						if len(explain) == 0 {
							explain = []string{"no functions consume"}
						}
						fm.cannotInclude = MustConsumeError{
							cause: fm.errorf("no consumer found for %s (%s)", t, strings.Join(explain, ", ")),
						}
						return fm.cannotInclude
					}
				}
			}
		}
		return nil
	}
}

func has(types []reflect.Type) func(reflect.Type) bool {
	m := make(map[reflect.Type]struct{})
	for _, typ := range types {
		m[typ] = struct{}{}
	}
	return func(typ reflect.Type) bool {
		_, ok := m[typ]
		return ok
	}
}

type MustConsumeError struct {
	cause error
}

func (e MustConsumeError) Error() string {
	return e.cause.Error()
}

func (e MustConsumeError) Unwrap() error {
	return e.cause
}
