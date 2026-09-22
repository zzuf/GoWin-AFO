package com

// noCopy is recognized by go vet's copylocks analysis.
type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}
