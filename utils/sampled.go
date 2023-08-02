package utils
type SampledLevel int
const (
    TimeModel SampledLevel = iota
    BBSampled
    WfSampled
    KernelSampled
    SampledLevelCount
)
