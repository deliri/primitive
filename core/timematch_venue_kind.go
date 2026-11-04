package core

// TimeMatchAppointmentVenueKind distinguishes a supplied destination from a
// request to create a video call on the owned organizer calendar event.
type TimeMatchAppointmentVenueKind uint8

const (
	TimeMatchVenueSupplied TimeMatchAppointmentVenueKind = iota
	TimeMatchVenueAutomaticVideo
)

func (k TimeMatchAppointmentVenueKind) Validate() error {
	switch k {
	case TimeMatchVenueSupplied, TimeMatchVenueAutomaticVideo:
		return nil
	default:
		return ErrTimeMatchContract
	}
}

func (TimeMatchAppointmentVenueKind) OffWireEnum() {}

var (
	_ Validatable = TimeMatchVenueSupplied
	_ OffWireEnum = TimeMatchVenueSupplied
)

func (k TimeMatchAppointmentVenueKind) IsValid() bool { return k.Validate() == nil }

func (k TimeMatchAppointmentVenueKind) String() string {
	switch k {
	case TimeMatchVenueSupplied:
		return "supplied"
	case TimeMatchVenueAutomaticVideo:
		return "automatic-video"
	default:
		return UnknownEnumDiagnostic
	}
}
