package base

import (
	"context"
	"fmt"

	"github.com/mjun0812/github-metrics/internal/format"
	"github.com/mjun0812/github-metrics/internal/plugins"
	"github.com/mjun0812/github-metrics/internal/templates"
	"github.com/mjun0812/github-metrics/internal/templates/chrome"
	"github.com/mjun0812/github-metrics/internal/templates/classic/partials"
)

// Partial names registered into the classic static partial dispatcher.
// They mirror the names listed in
// assets/templates/classic/partials/_.json so the lookup in classic.go
// resolves to our functions.
const (
	PartialActivityCommunity = "base.activity+community"
	PartialRepositories      = "base.repositories"
)

// Inline octicon SVGs lifted verbatim from the upstream
// base.activity+community.ejs / base.repositories.ejs templates so the
// rendered output stays byte-equivalent to the legacy partial. These
// constants are intentionally one-per-row so future octicon swaps stay
// reviewable.
const (
	octChart        = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M1.5 1.75a.75.75 0 00-1.5 0v12.5c0 .414.336.75.75.75h14.5a.75.75 0 000-1.5H1.5V1.75zm14.28 2.53a.75.75 0 00-1.06-1.06L10 7.94 7.53 5.47a.75.75 0 00-1.06 0L3.22 8.72a.75.75 0 001.06 1.06L7 7.06l2.47 2.47a.75.75 0 001.06 0l5.25-5.25z"></path></svg>`
	octCommit       = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M10.5 7.75a2.5 2.5 0 11-5 0 2.5 2.5 0 015 0zm1.43.75a4.002 4.002 0 01-7.86 0H.75a.75.75 0 110-1.5h3.32a4.001 4.001 0 017.86 0h3.32a.75.75 0 110 1.5h-3.32z"></path></svg>`
	octPRReview     = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M2.5 1.75a.25.25 0 01.25-.25h8.5a.25.25 0 01.25.25v7.736a.75.75 0 101.5 0V1.75A1.75 1.75 0 0011.25 0h-8.5A1.75 1.75 0 001 1.75v11.5c0 .966.784 1.75 1.75 1.75h3.17a.75.75 0 000-1.5H2.75a.25.25 0 01-.25-.25V1.75zM4.75 4a.75.75 0 000 1.5h4.5a.75.75 0 000-1.5h-4.5zM4 7.75A.75.75 0 014.75 7h2a.75.75 0 010 1.5h-2A.75.75 0 014 7.75zm11.774 3.537a.75.75 0 00-1.048-1.074L10.7 14.145 9.281 12.72a.75.75 0 00-1.062 1.058l1.943 1.95a.75.75 0 001.055.008l4.557-4.45z"></path></svg>`
	octPullRequest  = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M7.177 3.073L9.573.677A.25.25 0 0110 .854v4.792a.25.25 0 01-.427.177L7.177 3.427a.25.25 0 010-.354zM3.75 2.5a.75.75 0 100 1.5.75.75 0 000-1.5zm-2.25.75a2.25 2.25 0 113 2.122v5.256a2.251 2.251 0 11-1.5 0V5.372A2.25 2.25 0 011.5 3.25zM11 2.5h-1V4h1a1 1 0 011 1v5.628a2.251 2.251 0 101.5 0V5A2.5 2.5 0 0011 2.5zm1 10.25a.75.75 0 111.5 0 .75.75 0 01-1.5 0zM3.75 12a.75.75 0 100 1.5.75.75 0 000-1.5z"></path></svg>`
	octIssue        = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path d="M8 9.5a1.5 1.5 0 100-3 1.5 1.5 0 000 3z"></path><path fill-rule="evenodd" d="M8 0a8 8 0 100 16A8 8 0 008 0zM1.5 8a6.5 6.5 0 1113 0 6.5 6.5 0 01-13 0z"></path></svg>`
	octComment      = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M2.75 2.5a.25.25 0 00-.25.25v7.5c0 .138.112.25.25.25h2a.75.75 0 01.75.75v2.19l2.72-2.72a.75.75 0 01.53-.22h4.5a.25.25 0 00.25-.25v-7.5a.25.25 0 00-.25-.25H2.75zM1 2.75C1 1.784 1.784 1 2.75 1h10.5c.966 0 1.75.784 1.75 1.75v7.5A1.75 1.75 0 0113.25 12H9.06l-2.573 2.573A1.457 1.457 0 014 13.543V12H2.75A1.75 1.75 0 011 10.25v-7.5z"></path></svg>`
	octCommunity    = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M1.326 1.973a1.2 1.2 0 011.49-.832c.387.112.977.307 1.575.602.586.291 1.243.71 1.7 1.296.022.027.042.056.061.084A13.22 13.22 0 018 3c.67 0 1.289.037 1.861.108l.051-.07c.457-.586 1.114-1.004 1.7-1.295a9.654 9.654 0 011.576-.602 1.2 1.2 0 011.49.832c.14.493.356 1.347.479 2.29.079.604.123 1.28.07 1.936.541.977.773 2.11.773 3.301C16 13 14.5 15 8 15s-8-2-8-5.5c0-1.034.238-2.128.795-3.117-.08-.712-.034-1.46.052-2.12.122-.943.34-1.797.479-2.29zM8 13.065c6 0 6.5-2 6-4.27C13.363 5.905 11.25 5 8 5s-5.363.904-6 3.796c-.5 2.27 0 4.27 6 4.27z"></path><path d="M4 8a1 1 0 012 0v1a1 1 0 01-2 0V8zm2.078 2.492c-.083-.264.146-.492.422-.492h3c.276 0 .505.228.422.492C9.67 11.304 8.834 12 8 12c-.834 0-1.669-.696-1.922-1.508zM10 8a1 1 0 112 0v1a1 1 0 11-2 0V8z"></path></svg>`
	octOrgs         = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M1.5 14.25c0 .138.112.25.25.25H4v-1.25a.75.75 0 01.75-.75h2.5a.75.75 0 01.75.75v1.25h2.25a.25.25 0 00.25-.25V1.75a.25.25 0 00-.25-.25h-8.5a.25.25 0 00-.25.25v12.5zM1.75 16A1.75 1.75 0 010 14.25V1.75C0 .784.784 0 1.75 0h8.5C11.216 0 12 .784 12 1.75v12.5c0 .085-.006.168-.018.25h2.268a.25.25 0 00.25-.25V8.285a.25.25 0 00-.111-.208l-1.055-.703a.75.75 0 11.832-1.248l1.055.703c.487.325.779.871.779 1.456v5.965A1.75 1.75 0 0114.25 16h-3.5a.75.75 0 01-.197-.026c-.099.017-.2.026-.303.026h-3a.75.75 0 01-.75-.75V14h-1v1.25a.75.75 0 01-.75.75h-3zM3 3.75A.75.75 0 013.75 3h.5a.75.75 0 010 1.5h-.5A.75.75 0 013 3.75zM3.75 6a.75.75 0 000 1.5h.5a.75.75 0 000-1.5h-.5zM3 9.75A.75.75 0 013.75 9h.5a.75.75 0 010 1.5h-.5A.75.75 0 013 9.75zM7.75 9a.75.75 0 000 1.5h.5a.75.75 0 000-1.5h-.5zM7 6.75A.75.75 0 017.75 6h.5a.75.75 0 010 1.5h-.5A.75.75 0 017 6.75zM7.75 3a.75.75 0 000 1.5h.5a.75.75 0 000-1.5h-.5z"></path></svg>`
	octFollowing    = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M5.5 3.5a2 2 0 100 4 2 2 0 000-4zM2 5.5a3.5 3.5 0 115.898 2.549 5.507 5.507 0 013.034 4.084.75.75 0 11-1.482.235 4.001 4.001 0 00-7.9 0 .75.75 0 01-1.482-.236A5.507 5.507 0 013.102 8.05 3.49 3.49 0 012 5.5zM11 4a.75.75 0 100 1.5 1.5 1.5 0 01.666 2.844.75.75 0 00-.416.672v.352a.75.75 0 00.574.73c1.2.289 2.162 1.2 2.522 2.372a.75.75 0 101.434-.44 5.01 5.01 0 00-2.56-3.012A3 3 0 0011 4z"></path></svg>`
	octSponsoring   = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M4.25 2.5c-1.336 0-2.75 1.164-2.75 3 0 2.15 1.58 4.144 3.365 5.682A20.565 20.565 0 008 13.393a20.561 20.561 0 003.135-2.211C12.92 9.644 14.5 7.65 14.5 5.5c0-1.836-1.414-3-2.75-3-1.373 0-2.609.986-3.029 2.456a.75.75 0 01-1.442 0C6.859 3.486 5.623 2.5 4.25 2.5zM8 14.25l-.345.666-.002-.001-.006-.003-.018-.01a7.643 7.643 0 01-.31-.17 22.075 22.075 0 01-3.434-2.414C2.045 10.731 0 8.35 0 5.5 0 2.836 2.086 1 4.25 1 5.797 1 7.153 1.802 8 3.02 8.847 1.802 10.203 1 11.75 1 13.914 1 16 2.836 16 5.5c0 2.85-2.045 5.231-3.885 6.818a22.08 22.08 0 01-3.744 2.584l-.018.01-.006.003h-.002L8 14.25zm0 0l.345.666a.752.752 0 01-.69 0L8 14.25z"></path></svg>`
	octStar         = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M8 .25a.75.75 0 01.673.418l1.882 3.815 4.21.612a.75.75 0 01.416 1.279l-3.046 2.97.719 4.192a.75.75 0 01-1.088.791L8 12.347l-3.766 1.98a.75.75 0 01-1.088-.79l.72-4.194L.818 6.374a.75.75 0 01.416-1.28l4.21-.611L7.327.668A.75.75 0 018 .25zm0 2.445L6.615 5.5a.75.75 0 01-.564.41l-3.097.45 2.24 2.184a.75.75 0 01.216.664l-.528 3.084 2.769-1.456a.75.75 0 01.698 0l2.77 1.456-.53-3.084a.75.75 0 01.216-.664l2.24-2.183-3.096-.45a.75.75 0 01-.564-.41L8 2.694v.001z"></path></svg>`
	octEye          = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M1.679 7.932c.412-.621 1.242-1.75 2.366-2.717C5.175 4.242 6.527 3.5 8 3.5c1.473 0 2.824.742 3.955 1.715 1.124.967 1.954 2.096 2.366 2.717a.119.119 0 010 .136c-.412.621-1.242 1.75-2.366 2.717C10.825 11.758 9.473 12.5 8 12.5c-1.473 0-2.824-.742-3.955-1.715C2.92 9.818 2.09 8.69 1.679 8.068a.119.119 0 010-.136zM8 2c-1.981 0-3.67.992-4.933 2.078C1.797 5.169.88 6.423.43 7.1a1.619 1.619 0 000 1.798c.45.678 1.367 1.932 2.637 3.024C4.329 13.008 6.019 14 8 14c1.981 0 3.67-.992 4.933-2.078 1.27-1.091 2.187-2.345 2.637-3.023a1.619 1.619 0 000-1.798c-.45-.678-1.367-1.932-2.637-3.023C11.671 2.992 9.981 2 8 2zm0 8a2 2 0 100-4 2 2 0 000 4z"></path></svg>`
	octRepo         = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M2 2.5A2.5 2.5 0 014.5 0h8.75a.75.75 0 01.75.75v12.5a.75.75 0 01-.75.75h-2.5a.75.75 0 110-1.5h1.75v-2h-8a1 1 0 00-.714 1.7.75.75 0 01-1.072 1.05A2.495 2.495 0 012 11.5v-9zm10.5-1V9h-8c-.356 0-.694.074-1 .208V2.5a1 1 0 011-1h8zM5 12.25v3.25a.25.25 0 00.4.2l1.45-1.087a.25.25 0 01.3 0L8.6 15.7a.25.25 0 00.4-.2v-3.25a.25.25 0 00-.25-.25h-3.5a.25.25 0 00-.25.25z"></path></svg>`
	octLicense      = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M8.75.75a.75.75 0 00-1.5 0V2h-.984c-.305 0-.604.08-.869.23l-1.288.737A.25.25 0 013.984 3H1.75a.75.75 0 000 1.5h.428L.066 9.192a.75.75 0 00.154.838l.53-.53-.53.53v.001l.002.002.002.002.006.006.016.015.045.04a3.514 3.514 0 00.686.45A4.492 4.492 0 003 11c.88 0 1.556-.22 2.023-.454a3.515 3.515 0 00.686-.45l.045-.04.016-.015.006-.006.002-.002.001-.002L5.25 9.5l.53.53a.75.75 0 00.154-.838L3.822 4.5h.162c.305 0 .604-.08.869-.23l1.289-.737a.25.25 0 01.124-.033h.984V13h-2.5a.75.75 0 000 1.5h6.5a.75.75 0 000-1.5h-2.5V3.5h.984a.25.25 0 01.124.033l1.29.736c.264.152.563.231.868.231h.162l-2.112 4.692a.75.75 0 00.154.838l.53-.53-.53.53v.001l.002.002.002.002.006.006.016.015.045.04a3.517 3.517 0 00.686.45A4.492 4.492 0 0013 11c.88 0 1.556-.22 2.023-.454a3.512 3.512 0 00.686-.45l.045-.04.01-.01.006-.005.006-.006.002-.002.001-.002-.529-.531.53.53a.75.75 0 00.154-.838L13.823 4.5h.427a.75.75 0 000-1.5h-2.234a.25.25 0 01-.124-.033l-1.29-.736A1.75 1.75 0 009.735 2H8.75V.75zM1.695 9.227c.285.135.718.273 1.305.273s1.02-.138 1.305-.273L3 6.327l-1.305 2.9zm10 0c.285.135.718.273 1.305.273s1.02-.138 1.305-.273L13 6.327l-1.305 2.9z"></path></svg>`
	octRelease      = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M2.5 7.775V2.75a.25.25 0 01.25-.25h5.025a.25.25 0 01.177.073l6.25 6.25a.25.25 0 010 .354l-5.025 5.025a.25.25 0 01-.354 0l-6.25-6.25a.25.25 0 01-.073-.177zm-1.5 0V2.75C1 1.784 1.784 1 2.75 1h5.025c.464 0 .91.184 1.238.513l6.25 6.25a1.75 1.75 0 010 2.474l-5.026 5.026a1.75 1.75 0 01-2.474 0l-6.25-6.25A1.75 1.75 0 011 7.775zM6 5a1 1 0 100 2 1 1 0 000-2z"></path></svg>`
	octPackage      = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M8.878.392a1.75 1.75 0 00-1.756 0l-5.25 3.045A1.75 1.75 0 001 4.951v6.098c0 .624.332 1.2.872 1.514l5.25 3.045a1.75 1.75 0 001.756 0l5.25-3.045c.54-.313.872-.89.872-1.514V4.951c0-.624-.332-1.2-.872-1.514L8.878.392zM7.875 1.69a.25.25 0 01.25 0l4.63 2.685L8 7.133 3.245 4.375l4.63-2.685zM2.5 5.677v5.372c0 .09.047.171.125.216l4.625 2.683V8.432L2.5 5.677zm6.25 8.271l4.625-2.683a.25.25 0 00.125-.216V5.677L8.75 8.432v5.516z"></path></svg>`
	octDatabase     = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path xmlns="http://www.w3.org/2000/svg" fill-rule="evenodd" d="M2.5 3.5c0-.133.058-.318.282-.55.227-.237.592-.484 1.1-.708C4.899 1.795 6.354 1.5 8 1.5c1.647 0 3.102.295 4.117.742.51.224.874.47 1.101.707.224.233.282.418.282.551 0 .133-.058.318-.282.55-.227.237-.592.484-1.1.708C11.101 5.205 9.646 5.5 8 5.5c-1.647 0-3.102-.295-4.117-.742-.51-.224-.874-.47-1.101-.707-.224-.233-.282-.418-.282-.551zM1 3.5c0-.626.292-1.165.7-1.59.406-.422.956-.767 1.579-1.041C4.525.32 6.195 0 8 0c1.805 0 3.475.32 4.722.869.622.274 1.172.62 1.578 1.04.408.426.7.965.7 1.591v9c0 .626-.292 1.165-.7 1.59-.406.422-.956.767-1.579 1.041C11.476 15.68 9.806 16 8 16c-1.805 0-3.475-.32-4.721-.869-.623-.274-1.173-.62-1.579-1.04-.408-.426-.7-.965-.7-1.591v-9zM2.5 8V5.724c.241.15.503.286.779.407C4.525 6.68 6.195 7 8 7c1.805 0 3.475-.32 4.722-.869.275-.121.537-.257.778-.407V8c0 .133-.058.318-.282.55-.227.237-.592.484-1.1.708C11.101 9.705 9.646 10 8 10c-1.647 0-3.102-.295-4.117-.742-.51-.224-.874-.47-1.101-.707C2.558 8.318 2.5 8.133 2.5 8zm0 2.225V12.5c0 .133.058.318.282.55.227.237.592.484 1.1.708 1.016.447 2.471.742 4.118.742 1.647 0 3.102-.295 4.117-.742.51-.224.874-.47 1.101-.707.224-.233.282-.418.282-.551v-2.275c-.241.15-.503.285-.778.406-1.247.549-2.917.869-4.722.869-1.805 0-3.475-.32-4.721-.869a6.236 6.236 0 01-.779-.406z"></path></svg>`
	octHeartSponsor = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M4.25 2.5c-1.336 0-2.75 1.164-2.75 3 0 2.15 1.58 4.144 3.365 5.682A20.565 20.565 0 008 13.393a20.561 20.561 0 003.135-2.211C12.92 9.644 14.5 7.65 14.5 5.5c0-1.836-1.414-3-2.75-3-1.373 0-2.609.986-3.029 2.456a.75.75 0 01-1.442 0C6.859 3.486 5.623 2.5 4.25 2.5zM8 14.25l-.345.666-.002-.001-.006-.003-.018-.01a7.643 7.643 0 01-.31-.17 22.075 22.075 0 01-3.434-2.414C2.045 10.731 0 8.35 0 5.5 0 2.836 2.086 1 4.25 1 5.797 1 7.153 1.802 8 3.02 8.847 1.802 10.203 1 11.75 1 13.914 1 16 2.836 16 5.5c0 2.85-2.045 5.231-3.885 6.818a22.08 22.08 0 01-3.744 2.584l-.018.01-.006.003h-.002L8 14.25zm0 0l.345.666a.752.752 0 01-.69 0L8 14.25z"></path></svg>`
	octForker       = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M5 3.25a.75.75 0 11-1.5 0 .75.75 0 011.5 0zm0 2.122a2.25 2.25 0 10-1.5 0v.878A2.25 2.25 0 005.75 8.5h1.5v2.128a2.251 2.251 0 101.5 0V8.5h1.5a2.25 2.25 0 002.25-2.25v-.878a2.25 2.25 0 10-1.5 0v.878a.75.75 0 01-.75.75h-4.5A.75.75 0 015 6.25v-.878zm3.75 7.378a.75.75 0 11-1.5 0 .75.75 0 011.5 0zm3-8.75a.75.75 0 100-1.5.75.75 0 000 1.5z"></path></svg>`
	octViews        = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" width="16" height="16"><path fill-rule="evenodd" d="M0 1.75A.75.75 0 01.75 1h4.253c1.227 0 2.317.59 3 1.501A3.744 3.744 0 0111.006 1h4.245a.75.75 0 01.75.75v10.5a.75.75 0 01-.75.75h-4.507a2.25 2.25 0 00-1.591.659l-.622.621a.75.75 0 01-1.06 0l-.622-.621A2.25 2.25 0 005.258 13H.75a.75.75 0 01-.75-.75V1.75zm8.755 3a2.25 2.25 0 012.25-2.25H14.5v9h-3.757c-.71 0-1.4.201-1.992.572l.004-7.322zm-1.504 7.324l.004-5.073-.002-2.253A2.25 2.25 0 005.003 2.5H1.5v9h3.757a3.75 3.75 0 011.994.574z"></path></svg>`
)

func init() {
	partials.Register(PartialActivityCommunity, ActivityPartial)
	partials.Register(PartialRepositories, RepositoriesPartial)
}

// resolveResult fetches the base plugin's Result from Data. Returns
// (nil, false) when the caller should emit nothing — pc is missing,
// the plugin did not register a Result, or the Result is the wrong
// type / empty.
func resolveResult(pc *templates.PartialContext) (*Result, bool) {
	if pc == nil || pc.Data == nil {
		return nil, false
	}
	raw, ok := pc.Data.GetPlugin(Name)
	if !ok || raw == nil {
		return nil, false
	}
	r, ok := raw.(*Result)
	if !ok || r == nil {
		return nil, false
	}
	return r, true
}

// runEnabledForInputs reports whether the base plugin should perform
// any Provider fetch — true when any of the chrome panels it populates
// is opted into via `chrome_activity` / `chrome_community` /
// `chrome_repositories`, or when the legacy `plugin_base=yes` master
// switch is on (v2 compat — only honoured while no chrome_* input is
// declared).
func runEnabledForInputs(in map[string]any) bool {
	if chrome.TruthyInput(in, "chrome_activity") ||
		chrome.TruthyInput(in, "chrome_community") ||
		chrome.TruthyInput(in, "chrome_repositories") {
		return true
	}
	if !chrome.AnyChromeInputPresent(in) &&
		chrome.TruthyInput(in, "plugin_"+Name) {
		return true
	}
	return false
}

// activityEnabled reports whether the activity+community panel should
// render. The canonical surface is `chrome_activity` / `chrome_community`
// (#640); the legacy `plugin_base=yes` master switch still works as a
// compat shim while no chrome_* input is declared.
//
// Silence-by-default is preserved when no `chrome_*` input is
// declared and only `plugin_base=yes` is set — that combination keeps
// the panel hidden (matching the classic-octocat golden), so users on
// the legacy master switch see the same render they had under v2.
func activityEnabled(pc *templates.PartialContext) bool {
	if pc == nil {
		return false
	}
	if chrome.TruthyInput(pc.Inputs, "chrome_activity") ||
		chrome.TruthyInput(pc.Inputs, "chrome_community") {
		return true
	}
	if !chrome.AnyChromeInputPresent(pc.Inputs) &&
		chrome.TruthyInput(pc.Inputs, "plugin_"+Name) {
		return true
	}
	return false
}

// repositoriesEnabled reports whether the repositories summary panel
// should render. `chrome_repositories` is the canonical surface;
// `plugin_base=yes` alone (no chrome_* declared) is the legacy compat
// fallback.
func repositoriesEnabled(pc *templates.PartialContext) bool {
	if pc == nil {
		return false
	}
	if chrome.TruthyInput(pc.Inputs, "chrome_repositories") {
		return true
	}
	if !chrome.AnyChromeInputPresent(pc.Inputs) &&
		chrome.TruthyInput(pc.Inputs, "plugin_"+Name) {
		return true
	}
	return false
}

// ActivityPartial renders the activity + community two-column summary as
// native SVG (#409 Phase C: the outer foreignObject is gone, so the
// panel lays itself out and reports its own pixel height). Mirrors the
// deleted upstream base.activity+community.ejs (account === "user"
// branch). Renders nothing for organization profiles. Each column
// carries its own `<h2>` heading; the taller column sets the height.
func ActivityPartial(_ context.Context, pc *templates.PartialContext) (string, int, error) {
	if !activityEnabled(pc) {
		return "", 0, nil
	}
	r, ok := resolveResult(pc)
	if !ok {
		return "", 0, nil
	}
	if r.Profile == nil || r.Profile.Kind != plugins.ProfileKindUser || r.Profile.User == nil {
		return "", 0, nil
	}
	u := r.Profile.User

	// Activity column. Counters render unconditionally to mirror the
	// upstream EJS (which has no zero guard on these rows).
	left := chrome.NewSVGColumn(0, chrome.CardWidth/2, 0)
	left.Header(octChart, "Activity")
	left.Field(octCommit, countLabel(u.Commits, "Commit", "Commits"))
	left.Field(octPRReview, countLabel(u.PullRequestsReviewed,
		"Pull request reviewed", "Pull requests reviewed"))
	left.Field(octPullRequest, countLabel(u.PullRequestsOpened,
		"Pull request opened", "Pull requests opened"))
	left.Field(octIssue, countLabel(u.IssuesOpened,
		"Issue opened", "Issues opened"))
	left.Field(octComment, countLabel(u.IssueComments,
		"issue comment", "issue comments"))

	// Community column. Renders all rows unconditionally (matches
	// upstream which has no zero guards; empty accounts will see e.g.
	// "Member of 0 organizations" — same as the legacy behaviour).
	right := chrome.NewSVGColumn(chrome.CardWidth/2, chrome.CardWidth/2, 0)
	right.Header(octCommunity, "Community stats")
	right.Field(octOrgs, ofLabel("Member of", u.Organizations, "organization", "organizations"))
	right.Field(octFollowing, ofLabel("Following", u.Following, "user", "users"))
	right.Field(octSponsoring, ofLabel("Sponsoring", u.Sponsoring, "repository", "repositories"))
	right.Field(octStar, ofLabel("Starred", u.Starred, "repository", "repositories"))
	right.Field(octEye, ofLabel("Watching", u.Watching, "repository", "repositories"))

	height := int(maxF(left.Height(), right.Height()))
	return chrome.WrapSection("activity-community", height,
		left.Markup()+right.Markup()), height, nil
}

// RepositoriesPartial renders the repositories summary panel. Mirrors
// the deleted upstream base.repositories.ejs.
func RepositoriesPartial(_ context.Context, pc *templates.PartialContext) (string, int, error) {
	if !repositoriesEnabled(pc) {
		return "", 0, nil
	}
	r, ok := resolveResult(pc)
	if !ok {
		return "", 0, nil
	}
	if r.Profile == nil || r.Profile.Kind != plugins.ProfileKindUser || r.Profile.User == nil {
		return "", 0, nil
	}
	summary := r.RepositorySummary
	if summary == nil {
		summary = &plugins.ComputedRepositories{}
	}

	// Heading: "<N> Repositor[y/ies]", full-width. The upstream
	// parenthesised "(including <F> forks)" suffix was dropped
	// intentionally — the fork count is already surfaced in the Forkers
	// row below, and the duplicate phrasing crowded the heading on dense
	// profiles.
	head, headH := chrome.SVGSectionHeader(octRepo, repositoriesHeading(summary))
	colTop := headH

	// Left column: license / releases / packages / disk usage.
	left := chrome.NewSVGColumn(0, chrome.CardWidth/2, colTop)
	left.Field(octLicense, licensePreference(summary))
	left.Field(octRelease, countLabel(summary.Releases, "Release", "Releases"))
	left.Field(octPackage, countLabel(summary.Packages, "Package", "Packages"))
	left.Field(octDatabase, format.FormatDiskKB(summary.DiskUsage)+" used")

	// Right column: sponsors / stargazers / forkers / watchers / views.
	right := chrome.NewSVGColumn(chrome.CardWidth/2, chrome.CardWidth/2, colTop)
	right.Field(octHeartSponsor, countLabel(r.Profile.User.SponsorshipsAsMaintainer,
		"Sponsor", "Sponsors"))
	right.Field(octStar, countLabel(summary.Stargazers, "Stargazer", "Stargazers"))
	right.Field(octForker, countLabel(summary.Forks, "Forker", "Forkers"))
	right.Field(octEye, countLabel(summary.Watchers, "Watcher", "Watchers"))
	// Traffic views are surfaced inline at the bottom of the right
	// column to mirror upstream's base.repositories.ejs behaviour
	// (`<%= plugins.traffic.views.count %> view[s] in last two weeks`).
	// Read through the public TotalViews accessor so plugin_base does
	// not need to import internal/plugins/traffic and risk an init
	// cycle. Skipped / absent / zero-views all short-circuit silently.
	if rawTraffic, ok := pc.Data.GetPlugin("traffic"); ok && rawTraffic != nil {
		if tr, ok := rawTraffic.(interface{ TotalViews() int }); ok {
			if views := tr.TotalViews(); views > 0 {
				right.Field(octViews, viewsLabel(views))
			}
		}
	}

	height := int(colTop + maxF(left.Height(), right.Height()))
	return chrome.WrapSection("repositories", height,
		head+left.Markup()+right.Markup()), height, nil
}

// countLabel returns the "<count> <noun>" field label where `noun` is
// `singular` when count == 1 and `plural` otherwise. The count is
// rendered with the project's k/m/b short form via partials.FormatCount.
// The result is plain text handed to chrome.SVGText, which XML-escapes
// it, so no pre-escaping happens here.
func countLabel(count int, singular, plural string) string {
	noun := plural
	if count == 1 {
		noun = singular
	}
	return fmt.Sprintf("%s %s", partials.FormatCount(int64(count)), noun)
}

// viewsLabel returns the "<count> view[s] in last two weeks" field label
// surfaced in RepositoriesPartial's right column when the traffic plugin
// published a non-zero TotalViews. The pluralisation follows upstream's
// `s()` helper convention: "view" for count == 1, "views" otherwise.
func viewsLabel(count int) string {
	noun := "views"
	if count == 1 {
		noun = "view"
	}
	return fmt.Sprintf("%s %s in last two weeks", partials.FormatCount(int64(count)), noun)
}

// ofLabel returns the "<prefix> <count> <noun>" field label (e.g.
// "Member of 3 organizations") used by the community column, where the
// upstream EJS phrasing leads with the verb and inserts the count
// mid-sentence.
func ofLabel(prefix string, count int, singular, plural string) string {
	noun := plural
	if count == 1 {
		noun = singular
	}
	return fmt.Sprintf("%s %s %s", prefix, partials.FormatCount(int64(count)), noun)
}

// maxF returns the larger of two float64 column heights.
func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// repositoriesHeading returns "<N> Repositor[y/ies]" with Repository /
// Repositories pluralised on Count. The upstream "(including <F>
// forks)" suffix was dropped — ComputedRepositories.Forked is still
// populated and surfaced via the JSON envelope, but the heading no
// longer renders it.
func repositoriesHeading(s *plugins.ComputedRepositories) string {
	if s == nil {
		return "0 Repositories"
	}
	noun := "Repositories"
	if s.Count == 1 {
		noun = "Repository"
	}
	return fmt.Sprintf("%s %s", partials.FormatCount(int64(s.Count)), noun)
}

// licensePreference returns the "Prefers <license> license" label when
// the summary's top license bucket is non-empty, else "No license
// preference". Mirrors upstream
// `computed.licenses.favorite ? "Prefers <name>" : "No license preference"`.
func licensePreference(s *plugins.ComputedRepositories) string {
	if s == nil || len(s.LicensePreference) == 0 {
		return "No license preference"
	}
	top := s.LicensePreference[0]
	if top.Name == "" {
		return "No license preference"
	}
	// Plain text: chrome.SVGText XML-escapes the label at render time, so
	// no pre-escaping here (that would double-encode `<` / `&`).
	return "Prefers " + top.Name + " license"
}
