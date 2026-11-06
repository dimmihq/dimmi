//go:build !race

/*
|--------------------------------------------------------------------------
| Race Detector Off
|--------------------------------------------------------------------------
|
| Built without the race detector, where request timings are faithful
| enough to compare.
|
*/

package tests

const raceDetector = false
