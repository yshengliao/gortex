package context

// smartParams implements an optimized parameter storage
// It uses a small array for common cases (up to 4 params)
// and falls back to a map for larger cases
type smartParams struct {
	count    int
	keys     [4]string
	vals     [4]string  // Renamed to avoid conflict with values() method
	overflow map[string]string
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
	// Clear overflow map if it exists
	if sp.overflow != nil {
		for k := range sp.overflow {
			delete(sp.overflow, k)
		}
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
	
	// Check if key exists in overflow map
	if sp.overflow != nil {
		if _, exists := sp.overflow[key]; exists {
			sp.overflow[key] = value
			return
		}
	}
	
	// Add new parameter
	if sp.count < 4 {
		sp.keys[sp.count] = key
		sp.vals[sp.count] = value
		sp.count++
	} else {
		// Use overflow map for more than 4 params
		if sp.overflow == nil {
			sp.overflow = make(map[string]string)
		}
		sp.overflow[key] = value
		sp.count++
	}
}

// get retrieves a parameter value by key
func (sp *smartParams) get(key string) string {
	// Check array first
	for i := 0; i < sp.count && i < 4; i++ {
		if sp.keys[i] == key {
			return sp.vals[i]
		}
	}
	
	// Check overflow map
	if sp.overflow != nil {
		return sp.overflow[key]
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
	
	// For overflow items, we need to iterate
	// This is less efficient but only happens with many params
	if sp.overflow != nil {
		overflowIndex := index - 4
		current := 0
		for k, v := range sp.overflow {
			if current == overflowIndex {
				return k, v
			}
			current++
		}
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
	} else {
		// For overflow, we need to handle differently
		// This is a rare case and less efficient
		if sp.overflow == nil {
			sp.overflow = make(map[string]string)
		}
		sp.overflow[key] = value
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
	if sp.overflow != nil {
		for k := range sp.overflow {
			names = append(names, k)
		}
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
	if sp.overflow != nil {
		for _, v := range sp.overflow {
			values = append(values, v)
		}
	}
	
	return values
}