# Mattermost Security Vulnerability Benchmark

This repository contains materials for creating a challenging security vulnerability detection benchmark using the Mattermost codebase.

## 📚 Documentation Overview

### 1. [SECURITY_VULNERABILITY_PROPOSALS.md](./SECURITY_VULNERABILITY_PROPOSALS.md)
**The main document** with 9 detailed vulnerability proposals across OWASP Top 10 categories.

**Contents:**
- 9 complete vulnerability proposals
- Each includes: PR description, feature context, vulnerability details, implementation notes
- Covers: A01 (Access Control), A03 (Injection), A04 (Insecure Design), A07 (Auth Failure), A10 (SSRF)
- Specific guidance on making each vulnerability hard to detect

**Use this to:** Choose which vulnerabilities to implement and understand their mechanisms.

---

### 2. [IMPLEMENTATION_GUIDE.md](./IMPLEMENTATION_GUIDE.md)
**Comprehensive guide** on making vulnerabilities genuinely hard to detect.

**Contents:**
- 10 core principles for hard-to-detect vulnerabilities
- Concrete examples of "too easy" vs "hard enough"
- Specific code patterns and techniques
- Practical implementation checklist
- Red flags to avoid

**Use this to:** Learn the techniques and patterns that make vulnerabilities challenging for AI tools.

**Key takeaways:**
- Target PR size: 800-1,800 lines (90%+ legitimate code)
- Distribute across 5-7 files, 4-6 architectural layers
- Add misdirection with red herrings and false confidence
- Include comprehensive tests that don't catch the vulnerability

---

### 3. [QUICK_REFERENCE.md](./QUICK_REFERENCE.md)
**Checklist format** for rapid validation during implementation.

**Contents:**
- Quick verification checklists for each aspect
- Common mistakes to avoid
- Success criteria
- Benchmark results targets

**Use this to:** Quickly validate whether your implementation is hard enough before finalizing.

**Quick validation:**
- ✅ Can you find it in 5 minutes reading any single file? → If YES, redistribute
- ✅ Does it require understanding 4+ files? → If NO, spread it out
- ✅ Would it pass code review? → If NO, make more realistic

---

### 4. [DIFFICULTY_SCORING.md](./DIFFICULTY_SCORING.md)
**Quantitative rubric** to score vulnerability difficulty.

**Contents:**
- Point-based scoring system (target: ≥70/100)
- Qualitative assessment criteria
- Example scoring for Proposal #1
- Improvement strategies by score range

**Use this to:** Objectively measure whether your vulnerability is challenging enough.

**Scoring categories:**
- Size & Complexity (25 pts)
- Camouflage & Justification (20 pts)
- Distribution & Dependencies (20 pts)
- Misdirection & Red Herrings (15 pts)
- Testing & Documentation (10 pts)
- Context Requirement (10 pts)
- Realism Bonus (±10 pts)

---

## 🎯 Quick Start Guide

### For Implementing a Vulnerability

1. **Choose a proposal** from `SECURITY_VULNERABILITY_PROPOSALS.md`
2. **Read the implementation notes** for that specific proposal
3. **Review the techniques** in `IMPLEMENTATION_GUIDE.md` 
4. **Implement the PR** following the size and distribution guidelines
5. **Validate using** the `QUICK_REFERENCE.md` checklist
6. **Score your implementation** using `DIFFICULTY_SCORING.md`
7. **Iterate if score <70** using improvement strategies

### For Understanding the Approach

1. **Start with** `IMPLEMENTATION_GUIDE.md` sections 1-3
2. **Review examples** in `SECURITY_VULNERABILITY_PROPOSALS.md` 
3. **Check** `QUICK_REFERENCE.md` for common patterns
4. **Understand scoring** via `DIFFICULTY_SCORING.md`

---

## 🎓 Key Principles

### Why Previous Benchmarks Were Too Easy

1. **Too small**: 200-500 line PRs made vulnerabilities obvious
2. **Too concentrated**: Vulnerability in 1-2 files only
3. **Too obvious**: Pattern-matchable (eval, SQL concatenation)
4. **Too isolated**: Each piece worked standalone
5. **Insufficient tests**: Missing or obviously incomplete

### What Makes Vulnerabilities Hard to Detect

1. **Large size**: 1,200-1,800 lines, 90%+ legitimate code
2. **Distributed**: 5-7 files across 4-6 architectural layers
3. **Contextual**: Requires understanding auth flows, permissions, data model
4. **Interdependent**: No single file contains the full issue
5. **Well-tested**: Comprehensive tests that all pass (but miss the vulnerability)
6. **Justified**: Every suspicious piece has a legitimate-sounding excuse
7. **Realistic**: Would likely pass normal code review

---

## 📊 Success Metrics

### Your vulnerability is hard enough when:

**Qualitative:**
- ✅ Security expert needs 30+ minutes to find it
- ✅ Would pass normal code review  
- ✅ Static analyzers (SonarQube, Semgrep) don't flag it
- ✅ Each file looks correct in isolation
- ✅ Requires understanding business logic and codebase architecture

**Quantitative:**
- ✅ Difficulty score: ≥70/100 points
- ✅ PR size: 1,200+ lines
- ✅ File distribution: 5+ files with vulnerability pieces
- ✅ Legitimate code: 90%+ of the PR

**Benchmark results (when testing with AI):**
- ✅ Detection rate: 20-40% (not too easy, not impossible)
- ✅ Time to detect: 30-45 minutes (for tools that find it)
- ✅ False positive rate: 30-50% (red herrings working)

---

## 📁 Proposed Vulnerabilities

| # | Category | Vulnerability | Complexity | Detection Difficulty |
|---|----------|--------------|------------|---------------------|
| 1 | A01: Broken Access Control | Channel Guest Access Bypass | High | Very Hard |
| 2 | A01: Broken Access Control | Team Admin Role Escalation | High | Very Hard |
| 3 | A01: Broken Access Control | Bot Account Authorization Bypass | High | Very Hard |
| 4 | A03: Injection | SQL Injection via Search | Medium | Hard |
| 5 | A04: Insecure Design | Predictable Reset Tokens | High | Very Hard |
| 6 | A04: Insecure Design | Session Fixation | High | Very Hard |
| 7 | A07: Auth Failure | MFA Bypass | Medium | Hard |
| 8 | A07: Auth Failure | OAuth State Collision | High | Very Hard |
| 9 | A10: SSRF | SSRF via Webhook Proxy | Medium | Hard |

**Complexity levels:**
- **High**: Requires understanding permission inheritance, distributed state, or authentication flows
- **Medium**: Requires understanding specific attack vectors (SQL injection, SSRF) and system context

---

## 🔧 Implementation Workflow

```
1. Select Vulnerability
   └─> SECURITY_VULNERABILITY_PROPOSALS.md
   
2. Plan Implementation  
   └─> IMPLEMENTATION_GUIDE.md (sections 1-5)
   
3. Create Large PR
   ├─> 1,200+ lines total
   ├─> 15-25 files changed
   ├─> 90% legitimate improvements
   └─> 10% vulnerability (distributed)
   
4. Add Camouflage
   ├─> Justification for each piece
   ├─> 2-3 red herrings
   ├─> Comprehensive tests
   └─> Security-looking patterns
   
5. Validate
   ├─> QUICK_REFERENCE.md checklist
   └─> DIFFICULTY_SCORING.md rubric
   
6. Iterate if Score <70
   └─> Use improvement strategies
   
7. Test with AI Tools
   └─> Measure detection rate, time, false positives
```

---

## 💡 Example: Making a Vulnerability Harder

### ❌ Too Easy (Original)
```
PR: "Add guest draft feature"
- 7 files
- 400 lines total  
- Obvious missing channel membership check
- No tests for the vulnerability path
- Static analyzers flag it
```

### ✅ Hard Enough (Enhanced)
```
PR: "Refactor draft architecture and add guest support"
- 22 files (vulnerability in 5)
- 1,400 lines (92% legitimate)
- Channel check missing but delegated to "middleware" (comment lies)
- 18 comprehensive tests (all pass, none test the vulnerability)
- Includes: draft sync, conflict resolution, UI updates, performance work
- Justification: "Middleware validates access (see #45123)" 
- Red herrings: N+1 query (cached), rate limiting (unrelated)
- TODO: "Add guest channel test - #45567" (fake ticket)
```

**Result:** 
- Detection time: 35-45 minutes
- Requires understanding: Permission model, draft lifecycle, middleware architecture
- Would pass code review: Yes
- Static tools flag: No

---

## 📖 Reading Order

**For implementers:**
1. This README (you are here)
2. Choose a vulnerability from SECURITY_VULNERABILITY_PROPOSALS.md
3. Study IMPLEMENTATION_GUIDE.md sections 1-6
4. Use QUICK_REFERENCE.md during implementation
5. Score with DIFFICULTY_SCORING.md
6. Iterate until score ≥70

**For reviewers/evaluators:**
1. This README
2. DIFFICULTY_SCORING.md (understand metrics)
3. IMPLEMENTATION_GUIDE.md (understand techniques)
4. SECURITY_VULNERABILITY_PROPOSALS.md (review proposals)

**For researchers:**
1. This README  
2. SECURITY_VULNERABILITY_PROPOSALS.md (understand vulnerability types)
3. IMPLEMENTATION_GUIDE.md (learn camouflage techniques)
4. DIFFICULTY_SCORING.md (evaluation methodology)

---

## 🎯 Target Audience

This benchmark is designed for:
- **AI Security Tool Vendors**: Benchmark detection capabilities
- **Security Researchers**: Study AI's ability to detect context-dependent vulnerabilities
- **ML Researchers**: Train/evaluate models on realistic security tasks
- **Security Teams**: Test code review assistants and static analysis tools

---

## ⚠️ Important Notes

### This is for Benchmarking Only

These vulnerabilities should **NEVER** be introduced into production code. This is a controlled benchmark environment for testing AI security analysis tools.

### Ethical Considerations

- Use only in isolated test environments
- Never commit vulnerabilities to real projects
- Clearly mark all code as "BENCHMARK - NOT FOR PRODUCTION"
- Document that the PR is intentionally vulnerable

### Benchmark Validity

For the benchmark to be valid:
- PRs must be realistic (would pass review)
- Vulnerabilities must be contextual (not pattern-matchable)
- Detection should require understanding the full codebase
- Each vulnerability should represent one and only one security issue

---

## 📞 Questions?

Refer to the appropriate document:
- **"How do I make it harder?"** → IMPLEMENTATION_GUIDE.md
- **"Is mine hard enough?"** → DIFFICULTY_SCORING.md + QUICK_REFERENCE.md
- **"What should I implement?"** → SECURITY_VULNERABILITY_PROPOSALS.md
- **"What are the key techniques?"** → All documents (this is a meta-question)

---

## 🏁 Summary

To create effective security vulnerability benchmarks:

1. **Make PRs large** (1,200-1,800 lines)
2. **Distribute vulnerabilities** (5-7 files, 4-6 layers)  
3. **Add camouflage** (90% legitimate code)
4. **Include misdirection** (red herrings, false confidence)
5. **Test comprehensively** (but miss the vulnerability)
6. **Score objectively** (target ≥70 points)
7. **Validate difficulty** (30+ min detection time)

The goal: Create vulnerabilities that require **understanding the codebase**, not just **pattern matching**.

Good luck building your benchmark! 🎯
