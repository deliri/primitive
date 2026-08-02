package core

// ReaderConsecutiveEmptyReadMaximum is the common streaming refusal threshold
// for consecutive io.Reader results of (0, nil). The standard library permits
// that result transiently but defines io.ErrNoProgress for repeated instances;
// Primitive bounds the otherwise unending wait at this shared ceiling.
const ReaderConsecutiveEmptyReadMaximum = 100
