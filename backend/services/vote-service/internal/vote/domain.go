package vote

import "time"

const (
	visibilityPublic  = "public"
	visibilityPrivate = "private"

	licenseEveryone = "everyone"
	licenseInvited  = "invited_only"
	licenseGeoTime  = "geo_time"
)

type Event struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Visibility  string     `json:"visibility"`
	OwnerID     string     `json:"ownerId"`
	LicenseMode string     `json:"licenseMode"`
	GeoLat      *float64   `json:"geoLat,omitempty"`
	GeoLng      *float64   `json:"geoLng,omitempty"`
	GeoRadiusM  *int       `json:"geoRadiusM,omitempty"`
	VoteStart   *time.Time `json:"voteStart,omitempty"`
	VoteEnd     *time.Time `json:"voteEnd,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	IsJoined    bool       `json:"isJoined"`
}

type VoteResponse struct {
	Status     string `json:"status"`
	TrackID    string `json:"trackId"`
	TotalVotes int    `json:"totalVotes"`
}

type voteRequest struct {
	TrackID string   `json:"trackId"`
	Lat     *float64 `json:"lat,omitempty"`
	Lng     *float64 `json:"lng,omitempty"`
}

// err domain
type voteError struct {
	status int
	msg    string
}

func (e *voteError) Error() string {
	return e.msg
}

type inviteError struct {
	status int
	msg    string
}

func (e *inviteError) Error() string {
	return e.msg
}

type validationError struct {
	msg string
}

func (e *validationError) Error() string {
	return e.msg
}
