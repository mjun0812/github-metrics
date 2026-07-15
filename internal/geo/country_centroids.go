package geo

// countryCentroid is one row in the hand-maintained country → centroid
// table. Names lists the primary English name plus common formal /
// long variants (e.g. "Russia" and "Russian Federation") so lookups
// match whichever form appears in a GitHub profile location string.
type countryCentroid struct {
	Names []string
	ISO2  string
	ISO3  string
	Lat   float64
	Lng   float64
}

// countryCentroids is the compile-time table of centroids derived from
// Natural Earth 1:110m Admin 0 Countries (public domain / PDDL). Each
// centroid is the signed-area centroid of the country's largest
// polygon, so an entry such as "United States" sits near the middle of
// the contiguous 48 states rather than being pulled to the Aleutian
// side by Alaska. Coordinates are rounded to two decimals — the
// worldmap only needs point-level precision.
var countryCentroids = []countryCentroid{
	{Names: []string{"Fiji", "Republic of Fiji"}, ISO2: "FJ", ISO3: "FJI", Lat: -17.83, Lng: 178.0},
	{Names: []string{"United Republic of Tanzania", "Tanzania"}, ISO2: "TZ", ISO3: "TZA", Lat: -6.26, Lng: 34.75},
	{Names: []string{"Western Sahara", "Sahrawi Arab Democratic Republic"}, ISO2: "EH", ISO3: "ESH", Lat: 24.29, Lng: -12.14},
	{Names: []string{"Canada"}, ISO2: "CA", ISO3: "CAN", Lat: 57.75, Lng: -101.57},
	{Names: []string{"United States of America", "United States"}, ISO2: "US", ISO3: "USA", Lat: 39.5, Lng: -99.06},
	{Names: []string{"Kazakhstan", "Republic of Kazakhstan"}, ISO2: "KZ", ISO3: "KAZ", Lat: 48.19, Lng: 67.28},
	{Names: []string{"Uzbekistan", "Republic of Uzbekistan"}, ISO2: "UZ", ISO3: "UZB", Lat: 41.75, Lng: 63.2},
	{Names: []string{"Papua New Guinea", "Independent State of Papua New Guinea"}, ISO2: "PG", ISO3: "PNG", Lat: -6.65, Lng: 144.33},
	{Names: []string{"Indonesia", "Republic of Indonesia"}, ISO2: "ID", ISO3: "IDN", Lat: -0.25, Lng: 114.02},
	{Names: []string{"Argentina", "Argentine Republic"}, ISO2: "AR", ISO3: "ARG", Lat: -35.22, Lng: -65.15},
	{Names: []string{"Chile", "Republic of Chile"}, ISO2: "CL", ISO3: "CHL", Lat: -37.34, Lng: -71.67},
	{Names: []string{"Democratic Republic of the Congo"}, ISO2: "CD", ISO3: "COD", Lat: -2.85, Lng: 23.58},
	{Names: []string{"Somalia", "Federal Republic of Somalia"}, ISO2: "SO", ISO3: "SOM", Lat: 4.75, Lng: 45.73},
	{Names: []string{"Kenya", "Republic of Kenya"}, ISO2: "KE", ISO3: "KEN", Lat: 0.6, Lng: 37.79},
	{Names: []string{"Sudan", "Republic of the Sudan"}, ISO2: "SD", ISO3: "SDN", Lat: 15.99, Lng: 29.86},
	{Names: []string{"Chad", "Republic of Chad"}, ISO2: "TD", ISO3: "TCD", Lat: 15.33, Lng: 18.58},
	{Names: []string{"Haiti", "Republic of Haiti"}, ISO2: "HT", ISO3: "HTI", Lat: 18.9, Lng: -72.66},
	{Names: []string{"Dominican Republic"}, ISO2: "DO", ISO3: "DOM", Lat: 18.88, Lng: -70.46},
	{Names: []string{"Russia", "Russian Federation"}, ISO2: "RU", ISO3: "RUS", Lat: 61.69, Lng: 99.22},
	{Names: []string{"The Bahamas", "Bahamas", "Commonwealth of the Bahamas"}, ISO2: "BS", ISO3: "BHS", Lat: 24.51, Lng: -77.92},
	{Names: []string{"Falkland Islands", "Falkland Islands / Malvinas"}, ISO2: "FK", ISO3: "FLK", Lat: -51.71, Lng: -59.42},
	{Names: []string{"Norway", "Kingdom of Norway"}, ISO2: "-99", ISO3: "-99", Lat: 64.54, Lng: 14.24},
	{Names: []string{"Greenland"}, ISO2: "GL", ISO3: "GRL", Lat: 74.77, Lng: -41.5},
	{Names: []string{"French Southern and Antarctic Lands", "Territory of the French Southern and Antarctic Lands"}, ISO2: "TF", ISO3: "ATF", Lat: -49.31, Lng: 69.53},
	{Names: []string{"East Timor", "Timor-Leste", "Democratic Republic of Timor-Leste"}, ISO2: "TL", ISO3: "TLS", Lat: -8.77, Lng: 125.97},
	{Names: []string{"South Africa", "Republic of South Africa"}, ISO2: "ZA", ISO3: "ZAF", Lat: -28.96, Lng: 25.12},
	{Names: []string{"Lesotho", "Kingdom of Lesotho"}, ISO2: "LS", ISO3: "LSO", Lat: -29.63, Lng: 28.17},
	{Names: []string{"Mexico", "United Mexican States"}, ISO2: "MX", ISO3: "MEX", Lat: 23.94, Lng: -102.58},
	{Names: []string{"Uruguay", "Oriental Republic of Uruguay"}, ISO2: "UY", ISO3: "URY", Lat: -32.78, Lng: -56.0},
	{Names: []string{"Brazil", "Federative Republic of Brazil"}, ISO2: "BR", ISO3: "BRA", Lat: -10.81, Lng: -53.05},
	{Names: []string{"Bolivia", "Plurinational State of Bolivia"}, ISO2: "BO", ISO3: "BOL", Lat: -16.73, Lng: -64.64},
	{Names: []string{"Peru", "Republic of Peru"}, ISO2: "PE", ISO3: "PER", Lat: -9.19, Lng: -74.39},
	{Names: []string{"Colombia", "Republic of Colombia"}, ISO2: "CO", ISO3: "COL", Lat: 3.93, Lng: -73.08},
	{Names: []string{"Panama", "Republic of Panama"}, ISO2: "PA", ISO3: "PAN", Lat: 8.53, Lng: -80.11},
	{Names: []string{"Costa Rica", "Republic of Costa Rica"}, ISO2: "CR", ISO3: "CRI", Lat: 9.97, Lng: -84.18},
	{Names: []string{"Nicaragua", "Republic of Nicaragua"}, ISO2: "NI", ISO3: "NIC", Lat: 12.85, Lng: -85.02},
	{Names: []string{"Honduras", "Republic of Honduras"}, ISO2: "HN", ISO3: "HND", Lat: 14.82, Lng: -86.59},
	{Names: []string{"El Salvador", "Republic of El Salvador"}, ISO2: "SV", ISO3: "SLV", Lat: 13.73, Lng: -88.87},
	{Names: []string{"Guatemala", "Republic of Guatemala"}, ISO2: "GT", ISO3: "GTM", Lat: 15.7, Lng: -90.37},
	{Names: []string{"Belize"}, ISO2: "BZ", ISO3: "BLZ", Lat: 17.2, Lng: -88.7},
	{Names: []string{"Venezuela", "Bolivarian Republic of Venezuela"}, ISO2: "VE", ISO3: "VEN", Lat: 7.16, Lng: -66.16},
	{Names: []string{"Guyana", "Co-operative Republic of Guyana"}, ISO2: "GY", ISO3: "GUY", Lat: 4.79, Lng: -58.97},
	{Names: []string{"Suriname", "Republic of Suriname"}, ISO2: "SR", ISO3: "SUR", Lat: 4.12, Lng: -55.91},
	{Names: []string{"France", "French Republic"}, ISO2: "-99", ISO3: "-99", Lat: 46.61, Lng: 2.34},
	{Names: []string{"Ecuador", "Republic of Ecuador"}, ISO2: "EC", ISO3: "ECU", Lat: -1.45, Lng: -78.38},
	{Names: []string{"Puerto Rico", "Commonwealth of Puerto Rico"}, ISO2: "PR", ISO3: "PRI", Lat: 18.24, Lng: -66.48},
	{Names: []string{"Jamaica"}, ISO2: "JM", ISO3: "JAM", Lat: 18.14, Lng: -77.32},
	{Names: []string{"Cuba", "Republic of Cuba"}, ISO2: "CU", ISO3: "CUB", Lat: 21.63, Lng: -78.96},
	{Names: []string{"Zimbabwe", "Republic of Zimbabwe"}, ISO2: "ZW", ISO3: "ZWE", Lat: -18.91, Lng: 29.79},
	{Names: []string{"Botswana", "Republic of Botswana"}, ISO2: "BW", ISO3: "BWA", Lat: -22.1, Lng: 23.77},
	{Names: []string{"Namibia", "Republic of Namibia"}, ISO2: "NA", ISO3: "NAM", Lat: -22.1, Lng: 17.16},
	{Names: []string{"Senegal", "Republic of Senegal"}, ISO2: "SN", ISO3: "SEN", Lat: 14.35, Lng: -14.51},
	{Names: []string{"Mali", "Republic of Mali"}, ISO2: "ML", ISO3: "MLI", Lat: 17.27, Lng: -3.54},
	{Names: []string{"Mauritania", "Islamic Republic of Mauritania"}, ISO2: "MR", ISO3: "MRT", Lat: 20.21, Lng: -10.33},
	{Names: []string{"Benin", "Republic of Benin"}, ISO2: "BJ", ISO3: "BEN", Lat: 9.65, Lng: 2.34},
	{Names: []string{"Niger", "Republic of Niger"}, ISO2: "NE", ISO3: "NER", Lat: 17.35, Lng: 9.32},
	{Names: []string{"Nigeria", "Federal Republic of Nigeria"}, ISO2: "NG", ISO3: "NGA", Lat: 9.55, Lng: 8.0},
	{Names: []string{"Cameroon", "Republic of Cameroon"}, ISO2: "CM", ISO3: "CMR", Lat: 5.66, Lng: 12.61},
	{Names: []string{"Togo", "Togolese Republic"}, ISO2: "TG", ISO3: "TGO", Lat: 8.44, Lng: 1.0},
	{Names: []string{"Ghana", "Republic of Ghana"}, ISO2: "GH", ISO3: "GHA", Lat: 7.93, Lng: -1.24},
	{Names: []string{"Ivory Coast", "Côte d'Ivoire", "Republic of Ivory Coast"}, ISO2: "CI", ISO3: "CIV", Lat: 7.55, Lng: -5.61},
	{Names: []string{"Guinea", "Republic of Guinea"}, ISO2: "GN", ISO3: "GIN", Lat: 10.45, Lng: -11.06},
	{Names: []string{"Guinea-Bissau", "Republic of Guinea-Bissau"}, ISO2: "GW", ISO3: "GNB", Lat: 12.02, Lng: -15.11},
	{Names: []string{"Liberia", "Republic of Liberia"}, ISO2: "LR", ISO3: "LBR", Lat: 6.43, Lng: -9.41},
	{Names: []string{"Sierra Leone", "Republic of Sierra Leone"}, ISO2: "SL", ISO3: "SLE", Lat: 8.53, Lng: -11.8},
	{Names: []string{"Burkina Faso"}, ISO2: "BF", ISO3: "BFA", Lat: 12.31, Lng: -1.78},
	{Names: []string{"Central African Republic"}, ISO2: "CF", ISO3: "CAF", Lat: 6.54, Lng: 20.37},
	{Names: []string{"Republic of the Congo"}, ISO2: "CG", ISO3: "COG", Lat: -0.84, Lng: 15.13},
	{Names: []string{"Gabon", "Gabonese Republic"}, ISO2: "GA", ISO3: "GAB", Lat: -0.65, Lng: 11.69},
	{Names: []string{"Equatorial Guinea", "Republic of Equatorial Guinea"}, ISO2: "GQ", ISO3: "GNQ", Lat: 1.65, Lng: 10.37},
	{Names: []string{"Zambia", "Republic of Zambia"}, ISO2: "ZM", ISO3: "ZMB", Lat: -13.4, Lng: 27.73},
	{Names: []string{"Malawi", "Republic of Malawi"}, ISO2: "MW", ISO3: "MWI", Lat: -13.17, Lng: 34.19},
	{Names: []string{"Mozambique", "Republic of Mozambique"}, ISO2: "MZ", ISO3: "MOZ", Lat: -17.23, Lng: 35.47},
	{Names: []string{"eSwatini", "Kingdom of eSwatini"}, ISO2: "SZ", ISO3: "SWZ", Lat: -26.49, Lng: 31.4},
	{Names: []string{"Angola", "People's Republic of Angola"}, ISO2: "AO", ISO3: "AGO", Lat: -12.29, Lng: 17.5},
	{Names: []string{"Burundi", "Republic of Burundi"}, ISO2: "BI", ISO3: "BDI", Lat: -3.38, Lng: 29.91},
	{Names: []string{"Israel", "State of Israel"}, ISO2: "IL", ISO3: "ISR", Lat: 31.48, Lng: 35.0},
	{Names: []string{"Lebanon", "Lebanese Republic"}, ISO2: "LB", ISO3: "LBN", Lat: 33.91, Lng: 35.87},
	{Names: []string{"Madagascar", "Republic of Madagascar"}, ISO2: "MG", ISO3: "MDG", Lat: -19.36, Lng: 46.69},
	{Names: []string{"Palestine", "West Bank and Gaza"}, ISO2: "PS", ISO3: "PSE", Lat: 31.94, Lng: 35.27},
	{Names: []string{"Gambia", "The Gambia", "Republic of the Gambia"}, ISO2: "GM", ISO3: "GMB", Lat: 13.48, Lng: -15.43},
	{Names: []string{"Tunisia", "Republic of Tunisia"}, ISO2: "TN", ISO3: "TUN", Lat: 34.17, Lng: 9.53},
	{Names: []string{"Algeria", "People's Democratic Republic of Algeria"}, ISO2: "DZ", ISO3: "DZA", Lat: 28.19, Lng: 2.6},
	{Names: []string{"Jordan", "Hashemite Kingdom of Jordan"}, ISO2: "JO", ISO3: "JOR", Lat: 31.25, Lng: 36.78},
	{Names: []string{"United Arab Emirates"}, ISO2: "AE", ISO3: "ARE", Lat: 23.87, Lng: 54.21},
	{Names: []string{"Qatar", "State of Qatar"}, ISO2: "QA", ISO3: "QAT", Lat: 25.32, Lng: 51.18},
	{Names: []string{"Kuwait", "State of Kuwait"}, ISO2: "KW", ISO3: "KWT", Lat: 29.31, Lng: 47.6},
	{Names: []string{"Iraq", "Republic of Iraq"}, ISO2: "IQ", ISO3: "IRQ", Lat: 33.04, Lng: 43.76},
	{Names: []string{"Oman", "Sultanate of Oman"}, ISO2: "OM", ISO3: "OMN", Lat: 20.58, Lng: 56.1},
	{Names: []string{"Vanuatu", "Republic of Vanuatu"}, ISO2: "VU", ISO3: "VUT", Lat: -15.22, Lng: 166.91},
	{Names: []string{"Cambodia", "Kingdom of Cambodia"}, ISO2: "KH", ISO3: "KHM", Lat: 12.68, Lng: 104.88},
	{Names: []string{"Thailand", "Kingdom of Thailand"}, ISO2: "TH", ISO3: "THA", Lat: 15.02, Lng: 101.01},
	{Names: []string{"Laos", "Lao PDR", "Lao People's Democratic Republic"}, ISO2: "LA", ISO3: "LAO", Lat: 18.44, Lng: 103.75},
	{Names: []string{"Myanmar", "Republic of the Union of Myanmar"}, ISO2: "MM", ISO3: "MMR", Lat: 21.02, Lng: 96.51},
	{Names: []string{"Vietnam", "Socialist Republic of Vietnam"}, ISO2: "VN", ISO3: "VNM", Lat: 16.66, Lng: 106.29},
	{Names: []string{"North Korea", "Dem. Rep. Korea", "Democratic People's Republic of Korea"}, ISO2: "KP", ISO3: "PRK", Lat: 40.14, Lng: 127.17},
	{Names: []string{"South Korea", "Republic of Korea"}, ISO2: "KR", ISO3: "KOR", Lat: 36.43, Lng: 127.82},
	{Names: []string{"Mongolia"}, ISO2: "MN", ISO3: "MNG", Lat: 46.82, Lng: 102.95},
	{Names: []string{"India", "Republic of India"}, ISO2: "IN", ISO3: "IND", Lat: 22.93, Lng: 79.59},
	{Names: []string{"Bangladesh", "People's Republic of Bangladesh"}, ISO2: "BD", ISO3: "BGD", Lat: 23.84, Lng: 90.27},
	{Names: []string{"Bhutan", "Kingdom of Bhutan"}, ISO2: "BT", ISO3: "BTN", Lat: 27.43, Lng: 90.47},
	{Names: []string{"Nepal"}, ISO2: "NP", ISO3: "NPL", Lat: 28.24, Lng: 84.01},
	{Names: []string{"Pakistan", "Islamic Republic of Pakistan"}, ISO2: "PK", ISO3: "PAK", Lat: 29.97, Lng: 69.41},
	{Names: []string{"Afghanistan", "Islamic State of Afghanistan"}, ISO2: "AF", ISO3: "AFG", Lat: 33.86, Lng: 66.09},
	{Names: []string{"Tajikistan", "Republic of Tajikistan"}, ISO2: "TJ", ISO3: "TJK", Lat: 38.58, Lng: 71.03},
	{Names: []string{"Kyrgyzstan", "Kyrgyz Republic"}, ISO2: "KG", ISO3: "KGZ", Lat: 41.51, Lng: 74.62},
	{Names: []string{"Turkmenistan"}, ISO2: "TM", ISO3: "TKM", Lat: 39.09, Lng: 59.28},
	{Names: []string{"Iran", "Islamic Republic of Iran"}, ISO2: "IR", ISO3: "IRN", Lat: 32.52, Lng: 54.29},
	{Names: []string{"Syria", "Syrian Arab Republic"}, ISO2: "SY", ISO3: "SYR", Lat: 35.01, Lng: 38.54},
	{Names: []string{"Armenia", "Republic of Armenia"}, ISO2: "AM", ISO3: "ARM", Lat: 40.22, Lng: 45.0},
	{Names: []string{"Sweden", "Kingdom of Sweden"}, ISO2: "SE", ISO3: "SWE", Lat: 62.81, Lng: 16.6},
	{Names: []string{"Belarus", "Republic of Belarus"}, ISO2: "BY", ISO3: "BLR", Lat: 53.51, Lng: 27.98},
	{Names: []string{"Ukraine"}, ISO2: "UA", ISO3: "UKR", Lat: 49.15, Lng: 31.23},
	{Names: []string{"Poland", "Republic of Poland"}, ISO2: "PL", ISO3: "POL", Lat: 52.15, Lng: 19.31},
	{Names: []string{"Austria", "Republic of Austria"}, ISO2: "AT", ISO3: "AUT", Lat: 47.61, Lng: 14.08},
	{Names: []string{"Hungary", "Republic of Hungary"}, ISO2: "HU", ISO3: "HUN", Lat: 47.2, Lng: 19.36},
	{Names: []string{"Moldova", "Republic of Moldova"}, ISO2: "MD", ISO3: "MDA", Lat: 47.2, Lng: 28.41},
	{Names: []string{"Romania"}, ISO2: "RO", ISO3: "ROU", Lat: 45.86, Lng: 24.94},
	{Names: []string{"Lithuania", "Republic of Lithuania"}, ISO2: "LT", ISO3: "LTU", Lat: 55.28, Lng: 23.88},
	{Names: []string{"Latvia", "Republic of Latvia"}, ISO2: "LV", ISO3: "LVA", Lat: 56.81, Lng: 24.83},
	{Names: []string{"Estonia", "Republic of Estonia"}, ISO2: "EE", ISO3: "EST", Lat: 58.64, Lng: 25.82},
	{Names: []string{"Germany", "Federal Republic of Germany"}, ISO2: "DE", ISO3: "DEU", Lat: 51.13, Lng: 10.29},
	{Names: []string{"Bulgaria", "Republic of Bulgaria"}, ISO2: "BG", ISO3: "BGR", Lat: 42.75, Lng: 25.2},
	{Names: []string{"Greece", "Hellenic Republic"}, ISO2: "GR", ISO3: "GRC", Lat: 39.34, Lng: 22.56},
	{Names: []string{"Turkey", "Republic of Turkey"}, ISO2: "TR", ISO3: "TUR", Lat: 38.99, Lng: 35.39},
	{Names: []string{"Albania", "Republic of Albania"}, ISO2: "AL", ISO3: "ALB", Lat: 41.14, Lng: 20.03},
	{Names: []string{"Croatia", "Republic of Croatia"}, ISO2: "HR", ISO3: "HRV", Lat: 45.02, Lng: 16.57},
	{Names: []string{"Switzerland", "Swiss Confederation"}, ISO2: "CH", ISO3: "CHE", Lat: 46.79, Lng: 8.12},
	{Names: []string{"Luxembourg", "Grand Duchy of Luxembourg"}, ISO2: "LU", ISO3: "LUX", Lat: 49.77, Lng: 5.97},
	{Names: []string{"Belgium", "Kingdom of Belgium"}, ISO2: "BE", ISO3: "BEL", Lat: 50.65, Lng: 4.58},
	{Names: []string{"Netherlands", "Kingdom of the Netherlands"}, ISO2: "NL", ISO3: "NLD", Lat: 52.3, Lng: 5.51},
	{Names: []string{"Portugal", "Portuguese Republic"}, ISO2: "PT", ISO3: "PRT", Lat: 39.63, Lng: -8.06},
	{Names: []string{"Spain", "Kingdom of Spain"}, ISO2: "ES", ISO3: "ESP", Lat: 40.35, Lng: -3.62},
	{Names: []string{"Ireland"}, ISO2: "IE", ISO3: "IRL", Lat: 53.18, Lng: -8.01},
	{Names: []string{"New Caledonia"}, ISO2: "NC", ISO3: "NCL", Lat: -21.26, Lng: 165.53},
	{Names: []string{"Solomon Islands"}, ISO2: "SB", ISO3: "SLB", Lat: -7.9, Lng: 159.1},
	{Names: []string{"New Zealand"}, ISO2: "NZ", ISO3: "NZL", Lat: -43.99, Lng: 170.51},
	{Names: []string{"Australia", "Commonwealth of Australia"}, ISO2: "AU", ISO3: "AUS", Lat: -25.56, Lng: 134.38},
	{Names: []string{"Sri Lanka", "Democratic Socialist Republic of Sri Lanka"}, ISO2: "LK", ISO3: "LKA", Lat: 7.7, Lng: 80.67},
	{Names: []string{"China", "People's Republic of China"}, ISO2: "CN", ISO3: "CHN", Lat: 36.61, Lng: 103.87},
	{Names: []string{"Taiwan"}, ISO2: "CN-TW", ISO3: "TWN", Lat: 23.74, Lng: 120.97},
	{Names: []string{"Italy", "Italian Republic"}, ISO2: "IT", ISO3: "ITA", Lat: 43.47, Lng: 12.22},
	{Names: []string{"Denmark", "Kingdom of Denmark"}, ISO2: "DK", ISO3: "DNK", Lat: 56.22, Lng: 9.31},
	{Names: []string{"United Kingdom", "United Kingdom of Great Britain and Northern Ireland"}, ISO2: "GB", ISO3: "GBR", Lat: 53.88, Lng: -2.66},
	{Names: []string{"Iceland", "Republic of Iceland"}, ISO2: "IS", ISO3: "ISL", Lat: 65.07, Lng: -18.76},
	{Names: []string{"Azerbaijan", "Republic of Azerbaijan"}, ISO2: "AZ", ISO3: "AZE", Lat: 40.28, Lng: 47.68},
	{Names: []string{"Georgia"}, ISO2: "GE", ISO3: "GEO", Lat: 42.16, Lng: 43.48},
	{Names: []string{"Philippines", "Republic of the Philippines"}, ISO2: "PH", ISO3: "PHL", Lat: 15.75, Lng: 121.54},
	{Names: []string{"Malaysia"}, ISO2: "MY", ISO3: "MYS", Lat: 3.55, Lng: 114.68},
	{Names: []string{"Brunei", "Brunei Darussalam", "Negara Brunei Darussalam"}, ISO2: "BN", ISO3: "BRN", Lat: 4.69, Lng: 114.92},
	{Names: []string{"Slovenia", "Republic of Slovenia"}, ISO2: "SI", ISO3: "SVN", Lat: 46.13, Lng: 14.94},
	{Names: []string{"Finland", "Republic of Finland"}, ISO2: "FI", ISO3: "FIN", Lat: 64.5, Lng: 26.21},
	{Names: []string{"Slovakia", "Slovak Republic"}, ISO2: "SK", ISO3: "SVK", Lat: 48.73, Lng: 19.51},
	{Names: []string{"Czechia", "Czech Republic"}, ISO2: "CZ", ISO3: "CZE", Lat: 49.78, Lng: 15.33},
	{Names: []string{"Eritrea", "State of Eritrea"}, ISO2: "ER", ISO3: "ERI", Lat: 15.43, Lng: 38.68},
	{Names: []string{"Japan"}, ISO2: "JP", ISO3: "JPN", Lat: 36.02, Lng: 136.88},
	{Names: []string{"Paraguay", "Republic of Paraguay"}, ISO2: "PY", ISO3: "PRY", Lat: -23.25, Lng: -58.39},
	{Names: []string{"Yemen", "Republic of Yemen"}, ISO2: "YE", ISO3: "YEM", Lat: 15.91, Lng: 47.54},
	{Names: []string{"Saudi Arabia", "Kingdom of Saudi Arabia"}, ISO2: "SA", ISO3: "SAU", Lat: 24.12, Lng: 44.52},
	{Names: []string{"Antarctica"}, ISO2: "AQ", ISO3: "ATA", Lat: -80.52, Lng: 21.28},
	{Names: []string{"Northern Cyprus", "Turkish Republic of Northern Cyprus"}, ISO2: "-99", ISO3: "-99", Lat: 35.27, Lng: 33.56},
	{Names: []string{"Cyprus", "Republic of Cyprus"}, ISO2: "CY", ISO3: "CYP", Lat: 34.91, Lng: 33.04},
	{Names: []string{"Morocco", "Kingdom of Morocco"}, ISO2: "MA", ISO3: "MAR", Lat: 29.89, Lng: -8.42},
	{Names: []string{"Egypt", "Arab Republic of Egypt"}, ISO2: "EG", ISO3: "EGY", Lat: 26.51, Lng: 29.84},
	{Names: []string{"Libya"}, ISO2: "LY", ISO3: "LBY", Lat: 27.0, Lng: 17.97},
	{Names: []string{"Ethiopia", "Federal Democratic Republic of Ethiopia"}, ISO2: "ET", ISO3: "ETH", Lat: 8.65, Lng: 39.55},
	{Names: []string{"Djibouti", "Republic of Djibouti"}, ISO2: "DJ", ISO3: "DJI", Lat: 11.77, Lng: 42.5},
	{Names: []string{"Somaliland", "Republic of Somaliland"}, ISO2: "-99", ISO3: "-99", Lat: 9.76, Lng: 46.23},
	{Names: []string{"Uganda", "Republic of Uganda"}, ISO2: "UG", ISO3: "UGA", Lat: 1.3, Lng: 32.36},
	{Names: []string{"Rwanda", "Republic of Rwanda"}, ISO2: "RW", ISO3: "RWA", Lat: -2.01, Lng: 29.92},
	{Names: []string{"Bosnia and Herzegovina"}, ISO2: "BA", ISO3: "BIH", Lat: 44.18, Lng: 17.82},
	{Names: []string{"North Macedonia", "Republic of North Macedonia"}, ISO2: "MK", ISO3: "MKD", Lat: 41.61, Lng: 21.7},
	{Names: []string{"Republic of Serbia", "Serbia"}, ISO2: "RS", ISO3: "SRB", Lat: 44.23, Lng: 20.82},
	{Names: []string{"Montenegro"}, ISO2: "ME", ISO3: "MNE", Lat: 42.79, Lng: 19.29},
	{Names: []string{"Kosovo", "Republic of Kosovo"}, ISO2: "-99", ISO3: "-99", Lat: 42.58, Lng: 20.9},
	{Names: []string{"Trinidad and Tobago", "Republic of Trinidad and Tobago"}, ISO2: "TT", ISO3: "TTO", Lat: 10.43, Lng: -61.33},
	{Names: []string{"South Sudan", "Republic of South Sudan"}, ISO2: "SS", ISO3: "SSD", Lat: 7.29, Lng: 30.2},
}

// aliasCentroids lists extra name → row alias entries that Natural
// Earth's ADMIN / NAME_LONG / FORMAL_EN did not carry. Values reference
// countryCentroids indexes by ISO3.
var extraCountryAliases = map[string]string{
	"UK":                      "GBR",
	"U.K.":                    "GBR",
	"Britain":                 "GBR",
	"Great Britain":           "GBR",
	"England":                 "GBR",
	"Scotland":                "GBR",
	"Wales":                   "GBR",
	"Northern Ireland":        "GBR",
	"USA":                     "USA",
	"U.S.":                    "USA",
	"U.S.A.":                  "USA",
	"America":                 "USA",
	"South Korea":             "KOR",
	"Republic of Korea":       "KOR",
	"North Korea":             "PRK",
	"Russia":                  "RUS",
	"UAE":                     "ARE",
	"Vietnam":                 "VNM",
	"Ivory Coast":             "CIV",
	"Cape Verde":              "CPV",
	"Czechia":                 "CZE",
	"Burma":                   "MMR",
	"Congo":                   "COG",
	"DR Congo":                "COD",
	"DRC":                     "COD",
	"East Timor":              "TLS",
	"Timor Leste":             "TLS",
	"Palestinian Territories": "PSE",
	"Palestine":               "PSE",
	"Bosnia":                  "BIH",
	"Macedonia":               "MKD",
	"Netherlands":             "NLD",
	"Holland":                 "NLD",
	"Hong Kong":               "HKG",
	"Taiwan":                  "TWN",
	"Iran":                    "IRN",
	"Syria":                   "SYR",
	"Laos":                    "LAO",
	"Moldova":                 "MDA",
	"Brunei":                  "BRN",
	"Bolivia":                 "BOL",
	"Venezuela":               "VEN",
	"Tanzania":                "TZA",
	"Deutschland":             "DEU",
	"Nihon":                   "JPN",
	"Nippon":                  "JPN",
	"日本":                      "JPN",
}

// loadCountryCentroids populates g.countries with every name/alias in
// the compile-time tables so free-form location strings resolve to a
// stable centroid.
func (g *Geocoder) loadCountryCentroids() {
	byISO3 := make(map[string]Location, len(countryCentroids))
	for _, row := range countryCentroids {
		loc := Location{Lat: row.Lat, Lng: row.Lng, CountryCode: row.ISO2}
		if row.ISO3 != "" && row.ISO3 != "-99" {
			byISO3[row.ISO3] = loc
		}
		if row.ISO2 != "" && row.ISO2 != "-99" {
			g.countries[normalizeKey(row.ISO2)] = loc
		}
		if row.ISO3 != "" && row.ISO3 != "-99" {
			g.countries[normalizeKey(row.ISO3)] = loc
		}
		for _, name := range row.Names {
			key := normalizeKey(name)
			if key == "" {
				continue
			}
			g.countries[key] = loc
		}
	}
	for alias, iso3 := range extraCountryAliases {
		loc, ok := byISO3[iso3]
		if !ok {
			continue
		}
		key := normalizeKey(alias)
		if key == "" {
			continue
		}
		g.countries[key] = loc
	}
}
