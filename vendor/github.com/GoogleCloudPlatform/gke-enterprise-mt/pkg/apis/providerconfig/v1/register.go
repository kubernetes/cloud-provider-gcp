package v1

func init() {
	SchemeBuilder.Register(&ProviderConfig{}, &ProviderConfigList{})
}