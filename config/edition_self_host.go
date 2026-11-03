//go:build !cloud

/*
|--------------------------------------------------------------------------
| Self-Hosted Edition
|--------------------------------------------------------------------------
|
| The default build is the binary operators run themselves. It carries
| no cloud-only code, such as billing or the marketing site, and no
| setting can turn it into the cloud.
|
*/

package config

/*
BuiltEdition is the edition this binary was built as.
*/
const BuiltEdition = EditionSelfHost
