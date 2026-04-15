You are running the TrendPulse project quality check. Execute the following steps in order and report results:

1. **Build check**: Run `go build ./...` in `/home/desico/code/trendpulse`
   - Report: PASS or FAIL with error output

2. **Vet check**: Run `go vet ./...` in `/home/desico/code/trendpulse`
   - Report: PASS or FAIL with any warnings

3. **Test run**: Run `go test ./... -v -count=1` in `/home/desico/code/trendpulse`
   - Report: total tests, passed, failed
   - Show any failing test output

4. **Summary**: 
   - If all PASS: Output "✓ All checks passed"
   - If any FAIL: List what failed and suggest fixes

Do not modify any files — this is a read-only verification skill.
