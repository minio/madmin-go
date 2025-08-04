package madmin

type TierDebug struct {
	Bucket string `json:",omitempty"`
	Prefix string `json:",omitempty"`
}

func NewTierDebug(name, bucket, prefix string) (*TierConfig, error) {
	if name == "" {
		return nil, ErrTierNameEmpty
	}
	return &TierConfig{
		Name: name,
		Debug: &TierDebug{
			Bucket: bucket,
			Prefix: prefix,
		},
	}, nil
}
