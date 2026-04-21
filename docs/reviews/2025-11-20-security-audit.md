# Gortex Framework Security Audit Report

> **Status**: closed (2026-04-21). All 13 findings (1 CRITICAL, 4 HIGH,
> 6 MEDIUM, 2 LOW) are fixed on branch `claude/lucid-fermat-abeb8d`:
> path traversal, open redirect, CORS wildcard + credentials, dev error
> page redaction, JSON body-size limit, trusted-proxy client IP, JWT
> secret entropy, log body redaction, WebSocket read cap + authorizer,
> CSRF middleware, rate-limit headers, and configurable multipart limit
> are all landed. See [../../SECURITY.md](../../SECURITY.md) for reporting
> policy and defaults, and [./2025-11-20-code-review.md](./2025-11-20-code-review.md)
> for the companion review.

## Overview
Comprehensive security vulnerability assessment of the Gortex web framework codebase, focusing on authentication, middleware, WebSocket implementation, and input validation.

---

## CRITICAL VULNERABILITIES

### 1. PATH TRAVERSAL VULNERABILITY IN FILE SERVING
**Severity:** CRITICAL | **File:** `/home/user/gortex/transport/http/default.go` (Lines 380-407)

**Issue:**
The `File()` method accepts user-supplied file paths without validation or sanitization:

```go
func (c *DefaultContext) File(file string) error {
    f, err := os.Open(file)  // VULNERABLE: Direct path usage
    if err != nil {
        return err
    }
    defer f.Close()
    
    fi, err := f.Stat()
    if err != nil {
        return err
    }
    
    if fi.IsDir() {
        file = filepath.Join(file, "index.html")  // VULNERABLE: No validation
        f, err = os.Open(file)
        if err != nil {
            return err
        }
        defer f.Close()
        fi, err = f.Stat()
        if err != nil {
            return err
        }
    }
    
    http.ServeContent(c.response, c.request, fi.Name(), fi.ModTime(), f)
    return nil
}
```

**Attack Scenario:**
```
GET /file?path=../../../../etc/passwd
GET /file?path=/etc/shadow
```

An attacker can traverse the file system and access arbitrary files if the `File()` method is exposed to user input.

**Impact:** Disclosure of sensitive files, configuration files, private keys, source code

**Recommendation:**
- Implement path validation using `filepath.Clean()` and `filepath.Abs()`
- Enforce a base directory constraint
- Validate the resolved path is within the allowed directory:

```go
// Add to default.go
func validateFilePath(basePath, userPath string) (string, error) {
    cleanBase := filepath.Clean(basePath)
    cleanPath := filepath.Clean(userPath)
    absPath := filepath.Join(cleanBase, cleanPath)
    absPath, _ = filepath.Abs(absPath)
    
    if !strings.HasPrefix(absPath, cleanBase) {
        return "", fmt.Errorf("path traversal detected")
    }
    return absPath, nil
}
```

---

### 2. UNVALIDATED REDIRECT VULNERABILITY
**Severity:** HIGH | **File:** `/home/user/gortex/transport/http/default.go` (Lines 440-447)

**Issue:**
The `Redirect()` method accepts arbitrary URLs without validation:

```go
func (c *DefaultContext) Redirect(code int, url string) error {
    if code < 300 || code > 308 {
        return ErrInvalidRedirectCode
    }
    c.response.Header().Set(HeaderLocation, url)  // VULNERABLE: No URL validation
    c.response.WriteHeader(code)
    return nil
}
```

**Attack Scenario:**
```
POST /auth/redirect?target=https://evil.com/phishing
```

An attacker can redirect users to malicious sites for phishing attacks.

**Impact:** Phishing attacks, credential harvesting, malware distribution

**Recommendation:**
- Implement URL validation to allow only safe redirects:

```go
func isValidRedirect(url string) bool {
    // Only allow relative URLs or whitelisted domains
    if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
        // Validate against whitelist
        return false
    }
    if strings.HasPrefix(url, "//") {
        // Protocol-relative URLs can be dangerous
        return false
    }
    return strings.HasPrefix(url, "/") // Only allow relative paths
}
```

---

## HIGH SEVERITY VULNERABILITIES

### 3. CORS WILDCARD WITH CREDENTIALS
**Severity:** HIGH | **File:** `/home/user/gortex/middleware/cors.go` (Lines 86-89, 111-113)

**Issue:**
The CORS middleware allows wildcard origin (`*`) with credentials enabled, which violates CORS specification:

```go
// In CORSWithConfig:
resp.Header().Set("Access-Control-Allow-Origin", allowOrigin)  // Could be "*"
if config.AllowCredentials {
    resp.Header().Set("Access-Control-Allow-Credentials", "true")
}
```

**Default Config:**
```go
AllowOrigins:     []string{"*"},  // Line 29
AllowCredentials: false,          // Line 40 - but can be set to true by user
```

**Attack Scenario:**
If a developer sets `AllowOrigins: []string{"*"}` and `AllowCredentials: true`, the browser will reject the response, but misconfiguration could lead to security issues.

**Impact:** Potential CSRF attacks, cross-origin data leakage

**Recommendation:**
- Add validation to prevent wildcard with credentials:

```go
func (c *CORSConfig) Validate() error {
    for _, origin := range c.AllowOrigins {
        if origin == "*" && c.AllowCredentials {
            return fmt.Errorf("cannot use wildcard origin with AllowCredentials=true")
        }
    }
    return nil
}
```

---

### 4. SENSITIVE DATA EXPOSURE IN ERROR PAGES
**Severity:** HIGH | **File:** `/home/user/gortex/middleware/dev_error_page.go` (Lines 85-117)

**Issue:**
Development error pages expose sensitive information:

```go
if config.ShowRequestDetails {
    req := c.Request()
    errorInfo.RequestDetails = map[string]string{
        "method":      req.Method,
        "url":         req.URL.String(),        // VULNERABLE: Exposes full URL with query params
        "remote_addr": req.RemoteAddr,
        "user_agent":  req.UserAgent(),
        "referer":     req.Referer(),            // VULNERABLE: Leaks referring page
    }
    errorInfo.Headers = req.Header              // VULNERABLE: All headers exposed
}
```

**Information Disclosed:**
- Authorization headers (Bearer tokens, basic auth credentials)
- Session cookies
- API keys in URL parameters
- Internal service endpoints
- Request payloads

**Sample Output:**
```json
{
  "request_details": {
    "url": "https://api.example.com/api/users?token=sk_live_51234567890",
    "referer": "https://admin.internal.company.com/dashboard"
  },
  "headers": {
    "Authorization": "Bearer eyJhbGc..."
  }
}
```

**Impact:** Full authentication bypass, credential theft, internal service reconnaissance

**Recommendation:**
- Filter sensitive headers and parameters:

```go
var sensitiveHeaders = map[string]bool{
    "authorization": true,
    "cookie": true,
    "x-api-key": true,
    "x-auth-token": true,
}

var sensitiveParams = []string{"token", "password", "secret", "key", "apikey"}

func sanitizeHeaders(headers http.Header) map[string][]string {
    clean := make(map[string][]string)
    for k, v := range headers {
        if !sensitiveHeaders[strings.ToLower(k)] {
            clean[k] = v
        }
    }
    return clean
}
```

---

### 5. UNVALIDATED JSON DESERIALIZATION
**Severity:** HIGH | **File:** `/home/user/gortex/core/context/binder.go` (Lines 131-139)

**Issue:**
JSON body is deserialized without size limits:

```go
if c.Request().Header.Get("Content-Type") == "application/json" {
    if err := json.NewDecoder(c.Request().Body).Decode(structValue.Addr().Interface()); 
        err != nil && err.Error() != "EOF" {  // VULNERABLE: No size limit, ignores EOF errors
        // If JSON parsing fails, continue to try other binding methods
    }
}
```

**Problems:**
1. No request body size limit before decoding
2. Silent failure on EOF (error suppressed with string comparison)
3. Vulnerable to **DoS attacks** with large JSON payloads
4. Untrusted data decoded without limits

**Attack Scenario:**
```bash
# Send 1GB JSON file to exhaust memory
curl -X POST http://api.example.com/endpoint \
  -d @huge_payload.json \
  -H "Content-Type: application/json"
```

**Impact:** Denial of Service, memory exhaustion, server crash

**Recommendation:**
- Implement body size limits:

```go
// In binder.go
func (pb *ParameterBinder) bindStruct(c gortexContext.Context, structValue reflect.Value) error {
    // ... existing code ...
    
    const maxBodySize = 10 << 20 // 10MB
    if c.Request().Header.Get("Content-Type") == "application/json" {
        limitedBody := io.LimitReader(c.Request().Body, maxBodySize)
        if err := json.NewDecoder(limitedBody).Decode(structValue.Addr().Interface()); err != nil && err != io.EOF {
            return fmt.Errorf("JSON decode error: %w", err)
        }
    }
}
```

---

## MEDIUM SEVERITY VULNERABILITIES

### 6. CLIENT IP SPOOFING VIA HEADERS
**Severity:** MEDIUM | **File:** `/home/user/gortex/middleware/logger.go` (Lines 168-186)

**Issue:**
Client IP is extracted from user-controllable headers without validation:

```go
func getClientIP(req *http.Request) string {
    // Check X-Real-IP header
    if ip := req.Header.Get("X-Real-IP"); ip != "" {
        return ip  // VULNERABLE: User can spoof this
    }
    
    // Check X-Forwarded-For header
    if ip := req.Header.Get("X-Forwarded-For"); ip != "" {
        // Take the first IP if there are multiple
        if idx := bytes.IndexByte([]byte(ip), ','); idx >= 0 {
            return ip[:idx]
        }
        return ip  // VULNERABLE: User can spoof this
    }
    
    // Fall back to RemoteAddr
    return req.RemoteAddr
}
```

**Attack Scenario:**
An attacker can bypass IP-based rate limiting and geolocation restrictions:
```
X-Real-IP: 192.168.1.1
X-Forwarded-For: 192.168.1.1, 10.0.0.1
```

**Impact:** 
- Rate limit bypass
- Geolocation bypass
- Authentication bypass if IP is trusted
- Inaccurate audit logs

**Recommendation:**
- Only trust headers from known proxies:

```go
var trustedProxies = map[string]bool{
    "10.0.0.0/8": true,
    "172.16.0.0/12": true,
    "192.168.0.0/16": true,
}

func getClientIP(req *http.Request, trustedProxy bool) string {
    if !trustedProxy {
        return req.RemoteAddr
    }
    
    // Only then check forwarded headers
    if ip := req.Header.Get("X-Real-IP"); isValidIP(ip) {
        return ip
    }
    
    return req.RemoteAddr
}
```

---

### 7. WEAK SESSION ID VALIDATION
**Severity:** MEDIUM | **File:** `/home/user/gortex/middleware/auth.go` (Lines 306-324)

**Issue:**
Session middleware accepts session IDs from both cookies and headers without rate limiting or validation:

```go
func SessionAuthWithConfig(config *SessionConfig) MiddlewareFunc {
    // ... config setup ...
    
    return func(next HandlerFunc) HandlerFunc {
        return func(c Context) error {
            // ...
            // Get session ID from cookie or header
            sessionID := ""
            if cookie, err := req.Cookie(config.SessionKey); err == nil {
                sessionID = cookie.Value  // VULNERABLE: No validation
            }
            if sessionID == "" {
                sessionID = req.Header.Get(config.SessionKey)  // VULNERABLE: Can override cookie
            }
            
            if sessionID == "" {
                return &errors.ErrorResponse{...}
            }
            
            // Validate session - no rate limiting
            valid, err := config.SessionStore.Validate(sessionID)
            // ...
```

**Attack Scenario:**
Session enumeration/brute force with no rate limiting protection.

**Impact:** Session fixation, session hijacking, brute force attacks

**Recommendation:**
- Add rate limiting per session ID
- Implement session regeneration
- Add session timeout validation

---

### 8. SENSITIVE DATA IN LOGS
**Severity:** MEDIUM | **File:** `/home/user/gortex/middleware/logger.go` (Lines 75-128)

**Issue:**
Request bodies can be logged without sensitive data filtering:

```go
LoggerConfig struct {
    LogRequestBody  bool  // If true, logs request body
    LogResponseBody bool  // If true, logs response body
    BodyLogLimit    int   // Only limits size, not content type
}
```

If enabled, this logs:
- Passwords in POST bodies
- API keys in request payloads
- Credit card numbers
- Personal identification numbers

**Impact:** Sensitive data exposure in logs, credential compromise if logs are compromised

**Recommendation:**
- Implement sensitive field redaction:

```go
func redactSensitiveData(data []byte) []byte {
    sensitiveFields := []string{"password", "token", "key", "secret", "credit_card"}
    // Implement regex-based redaction
    return data
}
```

---

## LOW SEVERITY / BEST PRACTICE ISSUES

### 9. WEAK DEFAULT JWT SECRET HANDLING
**Severity:** LOW-MEDIUM | **File:** `/home/user/gortex/pkg/auth/jwt.go` (Lines 30-37)

**Issue:**
While the secret is configurable, there's no validation for minimum entropy:

```go
func NewJWTService(secretKey string, accessTTL, refreshTTL time.Duration, issuer string) *JWTService {
    return &JWTService{
        secretKey:       secretKey,  // VULNERABLE: No entropy check
        accessTokenTTL:  accessTTL,
        refreshTokenTTL: refreshTTL,
        issuer:          issuer,
    }
}
```

**Attack Scenario:**
Developers might use weak secrets like `"secret"`, `"password"`, or `"123456"`.

**Recommendation:**
- Add secret validation:

```go
func NewJWTService(secretKey string, accessTTL, refreshTTL time.Duration, issuer string) (*JWTService, error) {
    if len(secretKey) < 32 {
        return nil, fmt.Errorf("secret key must be at least 32 characters")
    }
    // ... rest of initialization
}
```

---

### 10. NO CSRF PROTECTION MECHANISM
**Severity:** MEDIUM | **File:** Framework-wide

**Issue:**
The framework doesn't provide CSRF token generation or validation middleware.

**Impact:** CSRF attacks on state-changing operations (POST, PUT, DELETE)

**Recommendation:**
- Implement CSRF middleware:

```go
type CSRFConfig struct {
    TokenLength int
    HeaderName  string
    CookieName  string
}

func CSRFMiddleware(config *CSRFConfig) MiddlewareFunc {
    // Generate tokens for GET/HEAD/OPTIONS
    // Validate tokens for POST/PUT/DELETE/PATCH
}
```

---

### 11. NO RATE LIMITING HEADERS
**Severity:** LOW | **File:** `/home/user/gortex/middleware/ratelimit.go`

**Issue:**
Rate limit responses don't include standard headers:

```go
// Missing headers in error response:
// X-RateLimit-Limit
// X-RateLimit-Remaining
// X-RateLimit-Reset
```

**Recommendation:**
- Add standard rate limit headers to responses

---

### 12. WEBSOCKET MESSAGE VALIDATION
**Severity:** MEDIUM | **File:** `/home/user/gortex/transport/websocket/client.go` (Lines 61-107)

**Issue:**
WebSocket messages are processed without authentication/authorization:

```go
for {
    var message Message
    err := c.conn.ReadJSON(&message)  // VULNERABLE: No size limit
    if err != nil {
        break
    }
    
    // Add client info to message
    message.ClientID = c.ID  // Message type could be spoofed
    
    switch message.Type {
    case "private":
        if target, ok := message.Data["target"].(string); ok {
            message.Target = target  // VULNERABLE: No validation
            c.hub.broadcast <- &message
        }
    // ...
}
```

**Attack Scenarios:**
1. Send large messages to cause DoS
2. Send unauthorized private messages
3. Impersonate other clients by setting target

**Recommendation:**
- Implement message size validation
- Add authorization checks
- Validate message types

---

### 13. MULTIPART FORM SIZE LIMIT
**Severity:** MEDIUM | **File:** `/home/user/gortex/transport/http/default.go` (Line 219)

**Issue:**
```go
func (c *DefaultContext) MultipartForm() (*multipart.Form, error) {
    err := c.request.ParseMultipartForm(32 << 20) // 32 MB - high default
    return c.request.MultipartForm, err
}
```

A 32MB limit for multipart forms is reasonable but should be configurable per-application.

---

## SUMMARY TABLE

| # | Vulnerability | Severity | File | Lines | Type |
|---|---|---|---|---|---|
| 1 | Path Traversal | CRITICAL | default.go | 380-407 | File Upload/Download |
| 2 | Unvalidated Redirect | HIGH | default.go | 440-447 | Open Redirect |
| 3 | CORS Wildcard + Creds | HIGH | cors.go | 86-113 | CORS |
| 4 | Sensitive Data in Errors | HIGH | dev_error_page.go | 85-117 | Information Disclosure |
| 5 | Unvalidated JSON | HIGH | binder.go | 131-139 | Deserialization DoS |
| 6 | IP Spoofing | MEDIUM | logger.go | 168-186 | IP Spoofing |
| 7 | Weak Session Validation | MEDIUM | auth.go | 306-324 | Session Management |
| 8 | Sensitive Logs | MEDIUM | logger.go | 75-128 | Information Disclosure |
| 9 | Weak JWT Secrets | MEDIUM | jwt.go | 30-37 | Weak Cryptography |
| 10 | No CSRF Protection | MEDIUM | Framework-wide | - | CSRF |
| 11 | No Rate Limit Headers | LOW | ratelimit.go | - | Best Practice |
| 12 | WebSocket Auth | MEDIUM | client.go | 61-107 | Authorization |
| 13 | High Multipart Limit | MEDIUM | default.go | 219 | DoS |

---

## RECOMMENDATIONS SUMMARY

### Immediate Actions (Critical):
1. Implement path validation for file serving
2. Add URL validation for redirects
3. Filter sensitive data from error pages in production

### Short Term (High Priority):
1. Add body size limits for JSON deserialization
2. Implement CSRF protection
3. Add WebSocket message validation
4. Implement proper IP validation for logging

### Medium Term:
1. Add configuration options for security settings
2. Implement security headers middleware
3. Add comprehensive logging of security events
4. Implement session security best practices

### Long Term:
1. Add comprehensive security testing in CI/CD
2. Implement security audit logging
3. Add rate limiting per user/session
4. Implement API key management

---

**Report Generated:** 2025-11-20
**Framework Version:** v0.4.0-alpha
**Assessment Status:** Complete
