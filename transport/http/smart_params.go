package http

// pathParam is a single key/value path parameter held in the overflow store.
type pathParam struct {
	key string
	val string
}

// smartParams implements an optimized parameter storage.
// It uses a small array for common cases (up to 4 params) and falls back to an
// ordered slice for larger cases. The slice (rather than a map) preserves
// insertion order so truncate can exactly drop the parameters added by an
// abandoned route branch during backtracking.
type smartParams struct {
	count    int
	keys     [4]string
	vals     [4]string
	overflow []pathParam
}

// newSmartParams creates a new smartParams instance
func newSmartParams() *smartParams {
	return &smartParams{}
}

// reset clears all parameters
func (sp *smartParams) reset() {
	sp.count = 0
	// Clear arrays
	for i := range sp.keys {
		sp.keys[i] = ""
		sp.vals[i] = ""
	}
	// Keep the backing array allocated but drop the entries.
	sp.overflow = sp.overflow[:0]
}

// truncate discards parameters beyond index n. Used for backtracking during
// route matching without allocating. Overflow entries live at logical index
// >= 4, so they are resliced to keep exactly the survivors; because the slice
// is insertion-ordered this is exact even for partial (n > 4) truncations.
func (sp *smartParams) truncate(n int) {
	sp.count = n
	switch {
	case n <= 4:
		sp.overflow = sp.overflow[:0]
	case n-4 < len(sp.overflow):
		sp.overflow = sp.overflow[:n-4]
	}
}

// set adds or updates a parameter
func (sp *smartParams) set(key, value string) {
	// Check if key already exists in array
	for i := 0; i < sp.count && i < 4; i++ {
		if sp.keys[i] == key {
			sp.vals[i] = value
			return
		}
	}

	// Check if key exists in overflow
	for i := range sp.overflow {
		if sp.overflow[i].key == key {
			sp.overflow[i].val = value
			return
		}
	}

	// Add new parameter
	if sp.count < 4 {
		sp.keys[sp.count] = key
		sp.vals[sp.count] = value
	} else {
		// Use overflow slice for more than 4 params
		sp.overflow = append(sp.overflow, pathParam{key: key, val: value})
	}
	sp.count++
}

// get retrieves a parameter value by key
func (sp *smartParams) get(key string) string {
	// Check array first
	for i := 0; i < sp.count && i < 4; i++ {
		if sp.keys[i] == key {
			return sp.vals[i]
		}
	}

	// Check overflow
	for i := range sp.overflow {
		if sp.overflow[i].key == key {
			return sp.overflow[i].val
		}
	}

	return ""
}

// getByIndex retrieves a parameter value by index
func (sp *smartParams) getByIndex(index int) (string, string) {
	if index < 0 || index >= sp.count {
		return "", ""
	}

	if index < 4 {
		return sp.keys[index], sp.vals[index]
	}

	overflowIndex := index - 4
	if overflowIndex < len(sp.overflow) {
		return sp.overflow[overflowIndex].key, sp.overflow[overflowIndex].val
	}

	return "", ""
}

// setByIndex sets a parameter by index
func (sp *smartParams) setByIndex(index int, key, value string) {
	if index < 0 || index >= sp.count {
		return
	}

	if index < 4 {
		sp.keys[index] = key
		sp.vals[index] = value
		return
	}

	overflowIndex := index - 4
	if overflowIndex < len(sp.overflow) {
		sp.overflow[overflowIndex] = pathParam{key: key, val: value}
	}
}

// names returns all parameter names
func (sp *smartParams) names() []string {
	if sp.count == 0 {
		return nil
	}

	names := make([]string, 0, sp.count)

	// Add array keys
	for i := 0; i < sp.count && i < 4; i++ {
		names = append(names, sp.keys[i])
	}

	// Add overflow keys
	for i := range sp.overflow {
		names = append(names, sp.overflow[i].key)
	}

	return names
}

// values returns all parameter values
func (sp *smartParams) values() []string {
	if sp.count == 0 {
		return nil
	}

	values := make([]string, 0, sp.count)

	// Add array values
	for i := 0; i < sp.count && i < 4; i++ {
		values = append(values, sp.vals[i])
	}

	// Add overflow values
	for i := range sp.overflow {
		values = append(values, sp.overflow[i].val)
	}

	return values
}
