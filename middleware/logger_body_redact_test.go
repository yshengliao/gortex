package middleware

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultBodyRedactorFlatObject(t *testing.T) {
	in := []byte(`{"username":"alice","password":"hunter2","token":"abc"}`)
	out := DefaultBodyRedactor(in)

	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	assert.Equal(t, "alice", got["username"])
	assert.Equal(t, bodyRedactionPlaceholder, got["password"])
	assert.Equal(t, bodyRedactionPlaceholder, got["token"])
}

func TestDefaultBodyRedactorCaseInsensitive(t *testing.T) {
	in := []byte(`{"API_KEY":"k","apiKey":"k2","CreditCard":"4111"}`)
	out := DefaultBodyRedactor(in)

	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	assert.Equal(t, bodyRedactionPlaceholder, got["API_KEY"])
	assert.Equal(t, bodyRedactionPlaceholder, got["apiKey"])
	assert.Equal(t, bodyRedactionPlaceholder, got["CreditCard"])
}

func TestDefaultBodyRedactorNestedObjectUnderSensitiveKey(t *testing.T) {
	in := []byte(`{"auth":{"username":"alice","secret":"s"},"token":{"value":"v","expires":"2030"}}`)
	out := DefaultBodyRedactor(in)

	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	// Everything under "token" is treated as sensitive because the
	// parent key itself matches the pattern; everything under "auth" is
	// recursively scanned and only "secret" redacted.
	tokenObj := got["token"].(map[string]any)
	assert.Equal(t, bodyRedactionPlaceholder, tokenObj["value"])
	assert.Equal(t, bodyRedactionPlaceholder, tokenObj["expires"])

	authObj := got["auth"].(map[string]any)
	assert.Equal(t, "alice", authObj["username"])
	assert.Equal(t, bodyRedactionPlaceholder, authObj["secret"])
}

func TestDefaultBodyRedactorArrayOfObjects(t *testing.T) {
	in := []byte(`{"items":[{"name":"a","password":"p1"},{"name":"b","password":"p2"}]}`)
	out := DefaultBodyRedactor(in)

	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	items := got["items"].([]any)
	require.Len(t, items, 2)
	first := items[0].(map[string]any)
	assert.Equal(t, "a", first["name"])
	assert.Equal(t, bodyRedactionPlaceholder, first["password"])
}

func TestDefaultBodyRedactorReturnsOriginalOnNonJSON(t *testing.T) {
	in := []byte(`this is not JSON password=secret`)
	out := DefaultBodyRedactor(in)
	assert.Equal(t, in, out)
}

func TestDefaultBodyRedactorReturnsOriginalOnEmpty(t *testing.T) {
	assert.Equal(t, []byte{}, DefaultBodyRedactor([]byte{}))
	assert.Equal(t, []byte(nil), DefaultBodyRedactor(nil))
}

func TestDefaultBodyRedactorLeavesNumericSecretsAlone(t *testing.T) {
	// By design — replacing numbers with strings would break downstream
	// log parsers. Callers needing numeric redaction can supply their
	// own BodyRedactor.
	in := []byte(`{"cvv":123,"password":"hidden"}`)
	out := DefaultBodyRedactor(in)

	var got map[string]any
	require.NoError(t, json.Unmarshal(out, &got))
	assert.Equal(t, float64(123), got["cvv"])
	assert.Equal(t, bodyRedactionPlaceholder, got["password"])
}
