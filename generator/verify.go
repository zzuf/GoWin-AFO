package generator

import (
	"context"

	internalverify "github.com/zzuf/GoWin-AFO/generator/internal/verify"
)

const ABIVerifierVersion = "0.1.0"

type ABIOracleInput struct {
	Architecture string
	Path         string
	Required     bool
}

type ABIVerifyOptions struct {
	Root          string
	Output        string
	SliceManifest string
	SourceLock    string
	ProbeManifest string
	OracleResults []ABIOracleInput
}

type ABIVerifyReport = internalverify.Report
type ABIVerificationError = internalverify.VerificationError

// VerifyABI builds the normalized vertical-slice IR without writing bindings,
// then validates pinned artifacts and compares independently measured ABI facts.
func VerifyABI(ctx context.Context, options ABIVerifyOptions) (ABIVerifyReport, error) {
	root, err := internalverify.FindRoot(options.Root)
	if err != nil {
		return ABIVerifyReport{}, err
	}
	generated, err := Generate(ctx, GenerateOptions{Root: root, Write: false})
	if err != nil {
		return ABIVerifyReport{}, err
	}
	oracles := make([]internalverify.OracleInput, len(options.OracleResults))
	for index, oracle := range options.OracleResults {
		oracles[index] = internalverify.OracleInput{Architecture: oracle.Architecture, Path: oracle.Path, Required: oracle.Required}
	}
	if options.OracleResults == nil {
		oracles = nil
	}
	return internalverify.Run(ctx, internalverify.Options{
		Root: root, Output: options.Output, SliceManifest: options.SliceManifest,
		SourceLock: options.SourceLock, ProbeManifest: options.ProbeManifest,
		OracleResults: oracles, Inventory: generated.Inventory,
		GeneratorVersion: Version, VerifierVersion: ABIVerifierVersion,
	})
}
