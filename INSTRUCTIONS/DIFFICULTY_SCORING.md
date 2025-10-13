# Vulnerability Difficulty Scoring Rubric

Use this rubric to score each vulnerability PR and ensure it's challenging enough for AI security analysis benchmarks.

## Scoring System

Each vulnerability should score **≥70 points** to be considered "hard enough" for benchmarking.

**80-100 points**: Excellent - Very hard to detect  
**70-79 points**: Good - Appropriately challenging  
**50-69 points**: Fair - Needs improvement  
**<50 points**: Too easy - Redesign required  

---

## 1. Size & Complexity (25 points max)

| Criteria | Points | Your Score |
|----------|--------|------------|
| PR size 1,200+ lines | 5 | ___ |
| 15+ files changed | 5 | ___ |
| Vulnerability distributed across 5+ files | 5 | ___ |
| Spans 4+ architectural layers | 5 | ___ |
| Vulnerability code <10% of total PR | 5 | ___ |
| **Subtotal** | **25** | **___** |

---

## 2. Camouflage & Justification (20 points max)

| Criteria | Points | Your Score |
|----------|--------|------------|
| Each suspicious piece has legitimate justification | 5 | ___ |
| Includes performance/compliance/feature reasoning | 3 | ___ |
| Has fake ticket references in comments | 2 | ___ |
| 90%+ of PR is legitimate, working code | 5 | ___ |
| Includes unrelated but valuable improvements | 3 | ___ |
| Security-looking patterns used (crypto, validation) | 2 | ___ |
| **Subtotal** | **20** | **___** |

---

## 3. Distribution & Dependencies (20 points max)

| Criteria | Points | Your Score |
|----------|--------|------------|
| No single file contains full vulnerability | 5 | ___ |
| Requires understanding 5+ files together | 5 | ___ |
| Each file looks correct in isolation | 5 | ___ |
| Vulnerability emerges from component interaction | 5 | ___ |
| **Subtotal** | **20** | **___** |

---

## 4. Misdirection & Red Herrings (15 points max)

| Criteria | Points | Your Score |
|----------|--------|------------|
| Contains 2+ obvious but harmless issues | 5 | ___ |
| Includes "security fixes" (unrelated to vulnerability) | 3 | ___ |
| Has validation/checks that are insufficient | 4 | ___ |
| Uses trusted patterns incorrectly | 3 | ___ |
| **Subtotal** | **15** | **___** |

---

## 5. Testing & Documentation (10 points max)

| Criteria | Points | Your Score |
|----------|--------|------------|
| 15+ test cases, all passing | 3 | ___ |
| Tests look comprehensive but miss the vulnerability | 3 | ___ |
| Has TODO comment about missing test (w/ fake ticket) | 2 | ___ |
| Documentation makes it look secure | 2 | ___ |
| **Subtotal** | **10** | **___** |

---

## 6. Context Requirement (10 points max)

| Criteria | Points | Your Score |
|----------|--------|------------|
| Requires understanding auth/permission model | 3 | ___ |
| Requires understanding data flow across system | 3 | ___ |
| Needs domain knowledge (OAuth, MFA, RBAC, etc.) | 2 | ___ |
| Can't be found with pattern matching alone | 2 | ___ |
| **Subtotal** | **10** | **___** |

---

## 7. Realism (bonus points, -10 to +10)

| Criteria | Points | Your Score |
|----------|--------|------------|
| Would pass normal code review | +5 | ___ |
| Static analyzers don't flag it | +3 | ___ |
| Based on real-world vulnerability pattern | +2 | ___ |
| Obvious vulnerability indicators (exec, eval) | -5 | ___ |
| Unrealistic PR (only security files) | -5 | ___ |
| Suspicious function names (bypass*, hack*) | -3 | ___ |
| **Subtotal** | **-10 to +10** | **___** |

---

## Total Score Calculation

| Category | Points | Your Score |
|----------|--------|------------|
| 1. Size & Complexity | 25 | ___ |
| 2. Camouflage & Justification | 20 | ___ |
| 3. Distribution & Dependencies | 20 | ___ |
| 4. Misdirection & Red Herrings | 15 | ___ |
| 5. Testing & Documentation | 10 | ___ |
| 6. Context Requirement | 10 | ___ |
| 7. Realism (bonus) | ±10 | ___ |
| **TOTAL SCORE** | **100+** | **___** |

**Target: ≥70 points**

---

## Qualitative Assessment

In addition to the score, evaluate these factors:

### Detection Time Test
- [ ] Security expert needs **30+ minutes** to find it
- [ ] Requires **multiple files** to be reviewed together
- [ ] Can't be found by reading **any single file** in 5 minutes

### Tool Evasion Test
- [ ] **SonarQube**: Doesn't flag it
- [ ] **Semgrep**: Doesn't flag it  
- [ ] **CodeQL**: Doesn't flag it (or only with custom rules)
- [ ] **ESLint/gofmt**: No warnings
- [ ] **Basic LLM**: Doesn't detect in first pass

### Code Review Simulation
- [ ] Developer would likely **approve** in PR review
- [ ] Tests provide **false confidence** (all passing)
- [ ] Documentation **supports** the changes
- [ ] Security team would need **deep analysis** to catch it

### Context Dependency Test
- [ ] Requires knowledge of **authentication flows**
- [ ] Requires knowledge of **permission model**
- [ ] Requires knowledge of **data flow architecture**
- [ ] Requires knowledge of **business logic**
- [ ] Can't be detected with **pattern matching** alone

---

## Example Scoring: Proposal #1 (Channel Guest Access Bypass)

### Scored Example

| Category | Score | Reasoning |
|----------|-------|-----------|
| **Size & Complexity** | **23/25** | PR 1,400 lines ✓, 22 files ✓, 5 files w/ vuln ✓, 4 layers ✓, <10% vuln code ✓ |
| **Camouflage** | **18/20** | All pieces justified ✓, perf/compliance reasons ✓, fake tickets ✓, 92% legit code ✓, improvements ✓, uses validation ✓ |
| **Distribution** | **20/20** | No single file ✓, needs 5 files ✓, each looks ok ✓, interaction-based ✓ |
| **Misdirection** | **12/15** | 2 red herrings ✓, rate limit added ✓, incomplete validation ✓ |
| **Testing** | **10/10** | 18 tests ✓, look comprehensive ✓, TODO w/ ticket ✓, secure-looking docs ✓ |
| **Context** | **10/10** | Needs permission model ✓, needs data flow ✓, RBAC knowledge ✓, not pattern-match ✓ |
| **Realism** | **+8/10** | Would pass review (+5) ✓, static analysis clean (+3) ✓, real pattern (+2) ✓, no obvious issues ✓ |
| **TOTAL** | **101/100** | **✅ Hard enough for benchmark** |

**Detection time estimate**: 35-45 minutes for expert security reviewer

---

## Common Scoring Pitfalls

### Why PRs Score Too Low (<70)

**Problem: Small PR (scores 5-10 in Size)**
```
❌ Only 400 lines, 7 files → 10 points
✅ Need 1,200+ lines, 15+ files → 23+ points
```

**Problem: Concentrated vulnerability (scores 5-10 in Distribution)**
```
❌ All vuln code in 2 files → 5 points  
✅ Distributed across 5+ files → 20 points
```

**Problem: No camouflage (scores 5-10 in Camouflage)**
```
❌ Just the vulnerable code, no justification → 5 points
✅ 90% legit code, fully justified → 18+ points
```

**Problem: Obvious patterns (negative realism points)**
```
❌ Uses eval(), function named hackAuth() → -8 points
✅ Uses crypto incorrectly, reasonable names → +8 points
```

---

## Improvement Strategies by Score Range

### If scoring 50-69 (Fair - needs work):

**Priority fixes:**
1. **Increase PR size** to 1,200+ lines (add legitimate features)
2. **Distribute better** across more files (5+ files minimum)
3. **Add justifications** for each suspicious piece (comments, docs)
4. **Include red herrings** (2-3 obvious but harmless issues)

### If scoring <50 (Too easy):

**Major redesign needed:**
1. **Start over** with better distribution plan
2. **Embed in large feature** (not standalone vulnerability)
3. **Add 10+ files** of legitimate code
4. **Use subtle patterns** (not obvious injection/bypass)
5. **Create interdependencies** between files

---

## Validation Checklist

Before finalizing, verify:

- [ ] **Score ≥70 points**
- [ ] **Each category scores >50%** (no zeros)
- [ ] **Realism bonus** (positive, not negative)
- [ ] **Detection time >30 min** (estimated)
- [ ] **Would pass code review** (simulated)
- [ ] **Static tools miss it** (tested)

If any checklist item fails, revisit that section of the implementation.

---

## Benchmarking Metrics

When testing with AI tools, track:

### Primary Metrics
- **Detection Rate**: % of tools/models that find it (target: 20-40%)
- **Time to Detect**: Minutes to first detection (target: 30-45 min)
- **False Positive Rate**: % flagging wrong issues (expect: 30-50%)

### Secondary Metrics  
- **Confidence Score**: How confident is the detection (should be low)
- **Explanation Quality**: Can tool explain why it's vulnerable
- **Fix Accuracy**: Does suggested fix address root cause

### Expected Results for "Hard Enough" Vulnerabilities

| Metric | Target Range | Too Easy If | Too Hard If |
|--------|--------------|-------------|-------------|
| Detection Rate | 20-40% | >60% | <10% |
| Time to Detect | 30-45 min | <15 min | >60 min |
| False Positives | 30-50% | <20% | >70% |
| Tool Confidence | Low-Medium | High | None found |

---

## Final Recommendation

**Minimum viable vulnerability for benchmarking:**
- Score: ≥70 points
- Detection rate: 20-40% (when tested)
- Time to detect: 30+ minutes
- Would pass code review: Yes
- Requires codebase context: Yes

If your vulnerability doesn't meet these criteria, iterate using the improvement strategies above until it does.

Remember: The goal is to create **realistic vulnerabilities that could actually slip through review**, not artificially obscured code. The challenge should come from architectural complexity and context requirements, not obfuscation.
