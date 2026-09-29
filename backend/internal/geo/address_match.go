package geo

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
)

// Photon returns nearby and fuzzy matches too. Only accept the complete
// requested city, street and house; a street centroid is not a house location.
type russianAddress struct {
	city, street, house string
	state               string
	ok                  bool
}

var (
	houseMarkerRE     = regexp.MustCompile(`(?i)(?:^|[ ,])(?:дом|д)\.?\s*((?:[0-9]|корпус|корп|к|строение|стр|с)[^;]*)$`)
	houseTailRE       = regexp.MustCompile(`(?i)(?:^|[ ,])([0-9]+(?:-[0-9]+)?[а-яa-z]?(?:\s*/\s*[0-9]+[а-яa-z]?)?(?:[\s,]*(?:корпус|корп|к|строение|стр|с)\.?\s*[0-9]+[а-яa-z]?){0,2})$`)
	housePartsRE      = regexp.MustCompile(`^([0-9]+(?:-[0-9]+)?[а-яa-z]?(?:/[0-9]+[а-яa-z]?)?)(?:к([0-9]+[а-яa-z]?))?(?:с([0-9]+[а-яa-z]?))?$`)
	standaloneHouseRE = regexp.MustCompile(`^[кс][0-9]+[а-я]?$`)
	streetMarkerRE    = regexp.MustCompile(`(?:^| )(?:улица|ул|пр-кт|пр-т|проспект|просп|проезд|переулок|пер|шоссе|ш|площадь|пл|набережная|наб|бульвар|бул|б-р) `)
	regionPrefixRE    = regexp.MustCompile(`(?i)^(?:мо|(?:обл\.?\s*)?московская область)\s*,\s*`)
	cityPrefixRE      = regexp.MustCompile(`(?i)^(?:г\.|город\s+|г\s+)`)
	unitMarkerRE      = regexp.MustCompile(`(?i)(?:[,;]|\s)(?:квартира|кв\.?|подъезд|этаж|офис|помещение|пом\.)\s*\d`)
)

func addressWords(s string) []string {
	s = strings.ReplaceAll(strings.ToLower(s), "ё", "е")
	return strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-'
	})
}

func normAddressPart(s string) string {
	words := addressWords(s)
	out := make([]string, 0, len(words))
	for _, word := range words {
		switch word {
		case "г", "город":
			continue
		case "ул":
			word = "улица"
		case "пр-кт", "пр-т", "просп":
			word = "проспект"
		case "пр-зд", "пр-д":
			word = "проезд"
		case "пер":
			word = "переулок"
		case "ш":
			word = "шоссе"
		case "пл":
			word = "площадь"
		case "наб":
			word = "набережная"
		case "бул", "б-р":
			word = "бульвар"
		}
		out = append(out, word)
	}
	return strings.Join(out, " ")
}

func normalizeOrdinal(word string) string {
	parts := strings.Split(word, "-")
	if len(parts) != 2 || !allDigits(parts[0]) {
		return word
	}
	switch parts[1] {
	case "й", "я", "е", "ый", "ой", "ая", "ое", "го":
		return parts[0]
	}
	return word
}

func normHouse(s string) string {
	s = strings.ReplaceAll(strings.ToLower(s), "ё", "е")
	s = strings.NewReplacer("корпус", "к", "корп", "к", "строение", "с", "стр", "с", ".", "", ",", "").Replace(s)
	s = strings.Join(strings.Fields(s), "")
	s = strings.NewReplacer("a", "а", "b", "в", "e", "е", "k", "к", "m", "м", "h", "н", "o", "о", "p", "р", "c", "с", "t", "т", "x", "х").Replace(s)
	if standaloneHouseRE.MatchString(s) {
		return s
	}
	parts := housePartsRE.FindStringSubmatch(s)
	if parts == nil {
		return ""
	}
	house, corpus := parts[1], parts[2]
	// Preserve a fractional house when the corpus is explicit: 83/2 к2.
	// Only corpus-free input uses the existing import shorthand 5/1 -> 5 к1.
	if before, after, fraction := strings.Cut(house, "/"); fraction && corpus == "" {
		house, corpus = before, after
	}
	if corpus != "" {
		house += " к" + corpus
	}
	if parts[3] != "" {
		house += " с" + parts[3]
	}
	return house
}

func parseRussianAddress(address string) russianAddress {
	raw := strings.TrimSpace(address)
	if unit := unitMarkerRE.FindStringIndex(raw); unit != nil {
		raw = raw[:unit[0]]
	}
	m := houseMarkerRE.FindStringSubmatchIndex(raw)
	if m == nil {
		m = houseTailRE.FindStringSubmatchIndex(raw)
	}
	if m == nil {
		return russianAddress{}
	}
	house := normHouse(raw[m[2]:m[3]])
	before := strings.Trim(raw[:m[0]], " ,;")
	before = regionPrefixRE.ReplaceAllString(before, "")
	explicitCity := cityPrefixRE.MatchString(before)
	segments := strings.Split(before, ",")
	var city, street string
	var state string
	if len(segments) >= 2 {
		// Administrative components are independent of the street. In
		// particular, do not interpret "region, city, settlement, street" as
		// one enormous street name.
		street = normAddressPart(segments[len(segments)-1])
		for _, segment := range segments[:len(segments)-1] {
			part := normAddressPart(segment)
			if part == "" || part == "россия" || part == "рф" || allDigits(part) {
				continue
			}
			if strings.Contains(part, "область") || strings.Contains(part, "край") || strings.Contains(part, "республика") {
				state = strings.TrimSpace(strings.TrimPrefix(part, "обл "))
				continue
			}
			if city == "" || cityPrefixRE.MatchString(strings.TrimSpace(segment)) {
				city = part
			}
		}
	} else {
		before = normAddressPart(before)
		words := strings.Fields(before)
		// Exported addresses also use "г. Кашира Центральная ул." and
		// "Москва Бирюлевская ул." with the street type after its name.
		if marker := streetMarkerRE.FindStringIndex(before); marker != nil {
			city, street = strings.TrimSpace(before[:marker[0]]), strings.TrimSpace(before[marker[0]:])
		} else if len(words) >= 3 && (explicitCity || words[0] == "москва") {
			city, street = words[0], strings.Join(words[1:], " ")
		}
	}
	if city == "" || street == "" || house == "" {
		return russianAddress{}
	}
	return russianAddress{city: city, street: street, house: house, state: state, ok: true}
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func addressMatches(_ string, wanted russianAddress, city, street, house string) bool {
	if !wanted.ok || normAddressPart(city) != wanted.city || normHouse(house) != wanted.house {
		return false
	}
	// Street type is optional in the input, but an explicitly supplied type
	// must match. Keep ordinals and every word of the actual street name.
	wantedName, wantedType := streetParts(wanted.street)
	actualName, actualType := streetParts(street)
	return wantedName != "" && wantedName == actualName && (wantedType == "" || wantedType == actualType)
}

func streetParts(street string) (string, string) {
	words := strings.Fields(normAddressPart(street))
	name := make([]string, 0, len(words))
	var kind string
	for _, word := range words {
		switch word {
		case "улица", "проспект", "проезд", "переулок", "шоссе", "площадь", "набережная", "бульвар":
			if kind != "" {
				return "", ""
			}
			kind = word
		default:
			name = append(name, normalizeOrdinal(word))
		}
	}
	sort.Strings(name)
	return strings.Join(name, " "), kind
}
