package domain

type NewPackage struct {
	Name        string
	Version     string
	NixMetadata NixMetadata
}

type Package struct {
	Name        string
	Version     string
	NixMetadata NixMetadata
}

type NixMetadata struct {
	StorePath string
	MainBin   string
}
