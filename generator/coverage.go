package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	internalcoverage "github.com/zzuf/GoWin-AFO/generator/internal/coverage"
)

type CoverageReport = internalcoverage.Report
type CoverageDelta = internalcoverage.Delta

func ReadCoverage(path string) (CoverageReport, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return CoverageReport{}, err
	}
	var r CoverageReport
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&r); err != nil {
		return r, err
	}
	var trailing any
	if err = dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("coverage report has trailing JSON")
		}
		return r, err
	}
	if err = internalcoverage.Validate(r); err != nil {
		return r, err
	}
	return r, nil
}
func DiffCoverage(a, b CoverageReport) CoverageDelta { return internalcoverage.Diff(a, b) }
func CheckCoverage(r CoverageReport, baseline *CoverageReport, failUnclassified, failRegression bool) error {
	return internalcoverage.Check(r, baseline, failUnclassified, failRegression)
}
func CoverageMarkdown(r CoverageReport) string { return internalcoverage.Markdown(r) }
func WriteCoverage(path string, r CoverageReport) error {
	if err := internalcoverage.Validate(r); err != nil {
		return err
	}
	b, err := internalcoverage.MarshalJSON(r)
	if err != nil {
		return err
	}
	return writeAtomicFile(path, b)
}
