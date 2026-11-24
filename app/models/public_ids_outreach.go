/*
|--------------------------------------------------------------------------
| Public ID Prefixes: Outreach
|--------------------------------------------------------------------------
|
| The prefixes of what a workspace sends on its own initiative:
| broadcasts and series. They are part of the one catalog in
| public_ids.go, kept here so that file stays readable.
|
*/

package models

/*
The prefixes of broadcasts, their topics and their recipients.
*/
const (
	PrefixBroadcast          Prefix = "bcst"
	PrefixBroadcastTopic     Prefix = "bctp"
	PrefixBroadcastRecipient Prefix = "bcrc"
)

/*
broadcastPublicIDs lists the prefixes of broadcasts, the topics people
subscribe to and each broadcast's recipients.
*/
func broadcastPublicIDs() []PublicIDEntry {
	return []PublicIDEntry{
		{PrefixBroadcast, "broadcasts"},
		{PrefixBroadcastTopic, "broadcast_topics"},
		{PrefixBroadcastRecipient, "broadcast_recipients"},
	}
}

/*
The prefixes of series, their versions and the journeys people take
through them.
*/
const (
	PrefixSeries           Prefix = "srs"
	PrefixSeriesVersion    Prefix = "srsv"
	PrefixSeriesEnrollment Prefix = "sren"
)

/*
seriesPublicIDs lists the prefixes of series, their published versions
and their enrollments.
*/
func seriesPublicIDs() []PublicIDEntry {
	return []PublicIDEntry{
		{PrefixSeries, "series"},
		{PrefixSeriesVersion, "series_versions"},
		{PrefixSeriesEnrollment, "series_enrollments"},
	}
}
