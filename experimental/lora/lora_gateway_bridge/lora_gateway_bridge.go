package lora_gateway_bridge

type BridgeMode uint8

const (
	BridgeModeDisabled BridgeMode = iota + 1
	BridgeModeGateway
)
