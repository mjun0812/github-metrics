// Package geo provides an offline, self-contained geocoder used by the
// stargazers worldmap variant. It embeds a trimmed copy of the GeoNames
// cities15000 dataset (CC BY 4.0) plus a hand-maintained table of
// country-name → country-centroid entries, so a location string reported
// by GitHub can be resolved to a (lat, lng) pair without touching the
// network. Attribution for GeoNames is recorded in THIRD_PARTY_LICENSES.md.
package geo
