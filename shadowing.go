package nject

import "fmt"

func checkForShadowing(funcs []*provider) error {
	returnedValues := make(map[typeCode]int)
	for i := len(funcs) - 1; i >= 0; i-- {
		fm := funcs[i]
		recevied := make(map[typeCode]bool)
		for _, tc := range fm.flows[receivedParams] {
			recevied[tc] = true
		}
		for _, tc := range fm.flows[returnParams] {
			if recevied[tc] {
				continue
			}
			from, ok := returnedValues[tc]
			if !ok {
				returnedValues[tc] = i
				continue
			}
			if (fm.class == fallibleStaticInjectorFunc || fm.class == fallibleInjectorFunc) &&
				(tc == errorTypeCode || tc == terminalErrorTypeCode) {
				returnedValues[tc] = i
				continue
			}
			if _, ok = fm.shadowingAllowed[tc]; ok {
				continue
			}
			return fmt.Errorf("%s returns %s overriding the return from %s, use AllowReturnShadowing to suppress this error", fm.String(), tc.String(), funcs[from].String())
		}
	}
	return nil
}

func mapCopy[K comparable, V any](m map[K]V) map[K]V {
	if m == nil {
		return nil
	}
	n := make(map[K]V)
	for k, v := range m {
		n[k] = v
	}
	return n
}
