package nject

// generateMustConsumeChecker only checks the down chain because
func generateMustConsumeChecker(funcs []*provider) {
	
}

func populateTransitiveRequire(funcs []*provider) {
	downSourceCount := make(map[reflect.Type]int)
	upSourceCount := make(map[reflect.Type]int)
	downSourceProvider := make(map[reflect.Type]int)
	upSourceProvider := make(map[reflect.Type]int)
	downConsumeCount := make(map[reflect.Type]int)
	upConsumeCount := make(map[reflect.Type]int)
	downConsumeProvider := make(map[reflect.Type]int)
	upConsumeProvider := make(map[reflect.Type]int)
	for _, fm := range funcs {
		recevied, returned := fm.UpFlows()
		for i, t := range returned {
			upSourceCount[t]++
			upSourceProvider[t] = i
		}
		for i, t := range recevied {
			upConsumeCount[t]++
			upConsumeProvider[t] = i
		}
		in_, out := range fm.DownFlows() 
		for i, t := range out {
			downSourceCount[t]++
			downSourceProvider[t] = i
		}
		for i, t := range in {
			downConsumeCount[t]++
			downConsumeProvider[t] = i
		}
	}
	deps := make(map[int][]int) // if key is required then list is required
	for i, fm := range funcs {
		recevied, returned := fm.UpFlows()
		for _, t := range recevied {
			if upSourceCount[t] == 1 {
				deps[i] = append(deps[i], upSourceProvider[t])
			}
		}
		if !fm.consumptionOptional {
			for _, t := range returned {
				if upConsumeCount[t] == 1 {
					deps[i] = append(deps[i], upConsumeProvider[t])
				}
			}
		}
		in_, out := range fm.DownFlows() 
		if fm.mustConsume {
			for _, t := range out {
				if downConsumeCount[t] == 1 {
					deps[i] = append(deps[i], downConsumeProvider[t])
				}
			}
		}
		for _, t := range in {
			if downSourceCount[t] == 1 {
				deps[i] = append(deps[i], downSourceProvider[t])
			}
		}
	}
	seen := make([]bool, len(funcs))
	todo := make([]int, 0, len(funcs))
	for i, fm := range funcs {
		if fm.required {
			todo = append(todo, i)
			seen[i] = true
			fm.transitiveRequire = fm.errorf("required")
		}
	}
	for len(todo) > 0 {
		i := todo[0]
		todo = todo[1:]
		if seen[i] { continue }
		seen[i] = true
		for _, d := range desp[i] {
			fm := funcs[d]
			if fm.transitiveRequire == nil {
				fm.transitiveRequire = fm.errorf("required to satisfy %s", funcs[i].transitiveRequire)
			}
		}
		todo = append(todo, deps[i]...)
	}
}
