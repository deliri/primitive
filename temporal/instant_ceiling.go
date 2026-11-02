package temporal

// Ceiling returns the first precision boundary at or after i. It uses signed
// remainder directly, so a representable ceiling near MinInt64 never requires
// constructing an unrepresentable preceding floor. Checked Add refuses a
// ceiling beyond the signed Instant range.
func (i Instant) Ceiling(precision Precision) (Instant, error) {
	if err := i.Validate(); err != nil {
		return Instant{}, err
	}
	unit, err := precision.nanoseconds()
	if err != nil {
		return Instant{}, err
	}
	remainder := i.nanoseconds % unit
	if remainder == 0 {
		return i, nil
	}
	adjustment := remainder
	if remainder > 0 {
		adjustment = unit - remainder
	}
	duration, err := DurationFromNanoseconds(adjustment)
	if err != nil {
		return Instant{}, err
	}
	return i.Add(duration)
}
