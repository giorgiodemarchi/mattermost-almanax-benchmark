# Quick Reference: Hard-to-Detect Vulnerability Checklist

Use this checklist when implementing each security vulnerability PR to ensure it's challenging enough for LLMs.

## ✅ PR Size & Structure

- [ ] **Total lines: 800-1,800** (target: 1,200-1,500)
- [ ] **Files changed: 15-25** (vulnerability in only 5-7 of them)
- [ ] **Vulnerability code: <10%** of total PR
- [ ] **90%+ is legitimate code** (refactoring, features, tests, docs)

## ✅ Vulnerability Distribution

- [ ] **Spans 4-6 architectural layers** (API → Service → Model → Store → UI)
- [ ] **Each file looks correct in isolation**
- [ ] **No single file contains the full vulnerability**
- [ ] **Requires understanding 5+ files together** to detect

## ✅ Camouflage Techniques

- [ ] **Legitimate justification** for each suspicious piece:
  - Performance optimization
  - Backward compatibility
  - Enterprise feature requirement
  - Migration/import support
  - Compliance requirement
  
- [ ] **Fake ticket references** in comments (#45123, JIRA-5678)
- [ ] **Security-looking patterns** that are actually broken:
  - Uses crypto library (but with weak input)
  - Has validation (but incomplete)
  - Checks permissions (but in wrong order)

## ✅ Red Herrings (Misdirection)

- [ ] **2-3 obvious but harmless issues**:
  - N+1 query (already cached)
  - Missing error handling (in non-critical path)
  - Unoptimized loop (in rarely-called code)
  
- [ ] **"Security fixes" included** (but unrelated to real vulnerability):
  - Add rate limiting
  - Add input sanitization
  - Update security docs
  
- [ ] **Obvious vulnerability that's actually safe**:
  - SQL without params (but for metadata only)
  - eval() (but with safe, controlled input)

## ✅ Test Strategy

- [ ] **15-25 test cases** total
- [ ] **All tests pass** ✓
- [ ] **Tests look comprehensive** (cover edge cases, errors, permissions)
- [ ] **Deliberately missing test** for the actual vulnerability
- [ ] **TODO comment** mentioning missing test with fake ticket reference

## ✅ Code Patterns to Use

### Instead of Obvious:
```go
❌ if user == "admin" { return true } // too obvious
```

### Use Distributed Logic:
```go
✅ File 1: if session.HasProp("bypass") { allowAccess() }
✅ File 2: func setBypass() { session.SetProp("bypass", "true") }  
✅ File 3: func initSession() { /* missing: clear bypass prop */ }
```

### Instead of No Validation:
```go
❌ query := "SELECT * FROM x WHERE " + userInput // obvious
```

### Use Insufficient Validation:
```go
✅ // Validation exists but has bypass
if containsKeywords(userInput, []string{"SELECT", "DROP"}) {
    return error // Case-sensitive, misses "select", "SeLeCt"
}
query := "SELECT * FROM x WHERE " + userInput
```

### Instead of Missing Permission Check:
```go
❌ func deleteUser(id) { db.Delete(id) } // obviously missing check
```

### Use Layered Trust:
```go
✅ File 1 (API): validateUserAuthenticated() ✓
✅ File 2 (Service): validateUserExists() ✓  
✅ File 3 (Service): /* trusts permission checked in API */ ✗
✅ File 4 (API): /* permission check was removed in refactoring */ ✗
```

## ✅ Specific Vulnerability Patterns

### Access Control Bypass
- [ ] Missing check distributed across layers
- [ ] Permission validation "delegated" to non-existent middleware
- [ ] Flag that enables bypass for "admin features"
- [ ] Trust boundary crossed without re-validation

### SQL Injection
- [ ] Parameterization used in 90% of queries
- [ ] Vulnerable query justified as "performance optimization"
- [ ] Validation regex that has bypasses
- [ ] JSON/XML path injection (less familiar)

### Authentication Bypass
- [ ] Global flag instead of session-specific
- [ ] Time limit that doesn't actually work (IsZero check)
- [ ] Recovery/backup mechanism that's too permissive
- [ ] Token with limited entropy hidden by caching justification

### SSRF
- [ ] Extensive URL validation (private IPs, localhost)
- [ ] Redirect following for "legitimate" reasons
- [ ] Validation only on initial URL, not redirects
- [ ] Debug flag that disables protection

## ✅ Comments & Documentation

- [ ] **Add "security" comments** that are misleading:
  - "Validated by middleware (see #ticket)"
  - "Permission check performed upstream"
  - "Input sanitized in previous step"
  
- [ ] **Performance justifications**:
  - "Cached for performance"
  - "Pre-computed to reduce latency"
  - "Optimized query building"
  
- [ ] **TODO comments with fake tickets**:
  - "// TODO: Add test coverage - JIRA-1234"
  - "// TODO: Validate in next PR - tracked in #5678"

## ✅ Final Validation

Before submitting, verify:

1. **Can you find the vulnerability in 5 minutes by reading any single file?**
   - If YES → Make it more distributed

2. **Does it require understanding 4+ files?**
   - If NO → Spread it out more

3. **Is there a believable justification for each suspicious piece?**
   - If NO → Add performance/feature/compliance justification

4. **Would a normal code review approve this?**
   - If NO → Make it more realistic

5. **Does the vulnerability require domain knowledge (auth flows, permissions, data model)?**
   - If NO → Make it more context-dependent

6. **Are there comprehensive tests that don't catch it?**
   - If NO → Add more tests (that pass)

7. **Static analysis tools (SonarQube, Semgrep) flag it?**
   - If YES → Make it less pattern-matchable

## 🎯 Success Criteria

Your PR is hard enough when:

✅ Security expert takes 30+ minutes to find it  
✅ Each file passes individual review  
✅ Static analyzers don't flag it  
✅ All tests pass  
✅ Justifications are believable  
✅ Requires understanding business logic  
✅ 90%+ of PR is legitimate code  
✅ Vulnerability emerges from component interactions  

## 📊 Benchmark Results Target

If testing with LLMs/AI tools:

- **Detection rate: 20-40%** (good difficulty)
- **Time to detect: 15-45 minutes** (for those that find it)
- **False positives: 1-3** (red herrings working)
- **Requires multiple analysis passes** (not found on first scan)

If detection rate is >60%, the vulnerabilities are too easy. Apply more camouflage techniques from this checklist.

## Common Mistakes (Too Easy)

❌ Small PR (< 500 lines)  
❌ Only security-related files changed  
❌ Obvious function names (`bypassSecurity()`)  
❌ Single file contains full vulnerability  
❌ No tests  
❌ Suspicious comments ("HACK", "TODO: fix security")  
❌ Pattern-matchable (eval, exec, + for SQL)  
❌ No legitimate justification  

Remember: The vulnerability should be **realistic** - something that could actually slip through code review in a real project.

