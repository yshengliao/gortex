package context

// SetParams efficiently sets multiple parameters at once
func SetParams(ctx Context, params map[string]string) {
	if dc, ok := ctx.(*DefaultContext); ok && len(params) > 0 {
		if dc.params == nil {
			dc.params = newSmartParams()
		}
		dc.params.reset()
		for key, value := range params {
			dc.params.set(key, value)
		}
	}
}