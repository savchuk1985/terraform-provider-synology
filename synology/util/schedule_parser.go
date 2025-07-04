package util

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// SpecSchedule specifies a duty cycle (to the second granularity), based on a
// traditional crontab specification. It is computed initially and stored as bit sets.
type Schedule struct {
	Second, Minute, Hour, Dom, Month, Dow, RepeatHour, RepeatMin, RepeatDate int64

	// Override location for this schedule.
	Location *time.Location
}

// bounds provides a range of acceptable values (plus a map of name to value).
type bounds struct {
	min, max int64
	names    map[string]int64
}

// The bounds for each field.
var (
	seconds = bounds{0, 59, nil}
	minutes = bounds{0, 59, nil}
	hours   = bounds{0, 23, nil}
	dom     = bounds{1, 31, nil}
	months  = bounds{1, 12, map[string]int64{
		"jan": 1,
		"feb": 2,
		"mar": 3,
		"apr": 4,
		"may": 5,
		"jun": 6,
		"jul": 7,
		"aug": 8,
		"sep": 9,
		"oct": 10,
		"nov": 11,
		"dec": 12,
	}}
	dow = bounds{0, 6, map[string]int64{
		"sun": 0,
		"mon": 1,
		"tue": 2,
		"wed": 3,
		"thu": 4,
		"fri": 5,
		"sat": 6,
	}}
)

const (
	// Set the top bit if a star was included in the expression.
	starBit = 1 << 63
)

// Configuration options for creating a parser. Most options specify which
// fields should be included, while others enable features. If a field is not
// included the parser will assume a default value. These options do not change
// the order fields are parse in.
type ParseOption int

const (
	Second      ParseOption = 1 << iota // Seconds field, default 0
	Minute                              // Minutes field, default 0
	Hour                                // Hours field, default 0
	Dom                                 // Day of month field, default *
	Month                               // Month field, default *
	Dow                                 // Day of week field, default *
	DowOptional                         // Optional day of week field, default *
	Descriptor                          // Allow descriptors such as @monthly, @weekly, etc.
)

var places = []ParseOption{
	Second,
	Minute,
	Hour,
	Dom,
	Month,
	Dow,
}

var defaults = []string{
	"0",
	"0",
	"0",
	"*",
	"*",
	"*",
}

// A custom Parser that can be configured.
type Parser struct {
	options   ParseOption
	optionals int
}

// Creates a custom Parser with custom options.
//
//	// Standard parser without descriptors
//	specParser := NewParser(Minute | Hour | Dom | Month | Dow)
//	sched, err := specParser.Parse("0 0 15 */3 *")
//
//	// Same as above, just excludes time fields
//	subsParser := NewParser(Dom | Month | Dow)
//	sched, err := specParser.Parse("15 */3 *")
//
//	// Same as above, just makes Dow optional
//	subsParser := NewParser(Dom | Month | DowOptional)
//	sched, err := specParser.Parse("15 */3")
func NewParser(options ParseOption) Parser {
	optionals := 0
	if options&DowOptional > 0 {
		options |= Dow
		optionals++
	}
	return Parser{options, optionals}
}

// Parse returns a new crontab schedule representing the given spec.
// It returns a descriptive error if the spec is not valid.
// It accepts crontab specs and features configured by NewParser.
func (p Parser) Parse(spec string) (*Schedule, error) {
	if len(spec) == 0 {
		return nil, fmt.Errorf("empty spec string")
	}
	if spec[0] == '@' && p.options&Descriptor > 0 {
		return parseDescriptor(spec)
	}

	// Figure out how many fields we need
	upperLimit := 0
	for _, place := range places {
		if p.options&place > 0 {
			upperLimit++
		}
	}
	lowerLimit := upperLimit - p.optionals

	// Split fields on whitespace
	fields := strings.Fields(spec)

	// Validate number of fields
	if count := len(fields); count < lowerLimit || count > upperLimit {
		if lowerLimit == upperLimit {
			return nil, fmt.Errorf(
				"expected exactly %d fields, found %d: %s",
				lowerLimit,
				count,
				spec,
			)
		}
		return nil, fmt.Errorf(
			"expected %d to %d fields, found %d: %s",
			lowerLimit,
			upperLimit,
			count,
			spec,
		)
	}

	// Fill in missing fields
	fields = expandFields(fields, p.options)

	var err error
	field := func(field string, r bounds) int64 {
		if err != nil {
			return 0
		}
		var bits int64
		bits, err = getField(field, r)
		return bits
	}

	var (
		second     = field(fields[0], seconds)
		minute     = field(fields[1], minutes)
		hour       = field(fields[2], hours)
		dayofmonth = field(fields[3], dom)
		month      = field(fields[4], months)
		dayofweek  = field(fields[5], dow)
	)
	if err != nil {
		return nil, err
	}

	return &Schedule{
		Second: second,
		Minute: minute,
		Hour:   hour,
		Dom:    dayofmonth,
		Month:  month,
		Dow:    dayofweek,
	}, nil
}

func expandFields(fields []string, options ParseOption) []string {
	n := 0
	count := len(fields)
	expFields := make([]string, len(places))
	copy(expFields, defaults)
	for i, place := range places {
		if options&place > 0 {
			expFields[i] = fields[n]
			n++
		}
		if n == count {
			break
		}
	}
	return expFields
}

var standardParser = NewParser(
	Minute | Hour | Dom | Month | Dow | Descriptor,
)

// ParseStandard returns a new crontab schedule representing the given standardSpec
// (https://en.wikipedia.org/wiki/Cron). It differs from Parse requiring to always
// pass 5 entries representing: minute, hour, day of month, month and day of week,
// in that order. It returns a descriptive error if the spec is not valid.
//
// It accepts
//   - Standard crontab specs, e.g. "* * * * ?"
//   - Descriptors, e.g. "@midnight", "@every 1h30m"
func ParseStandard(standardSpec string) (*Schedule, error) {
	return standardParser.Parse(standardSpec)
}

var defaultParser = NewParser(
	Second | Minute | Hour | Dom | Month | DowOptional | Descriptor,
)

// Parse returns a new crontab schedule representing the given spec.
// It returns a descriptive error if the spec is not valid.
//
// It accepts
//   - Full crontab specs, e.g. "* * * * * ?"
//   - Descriptors, e.g. "@midnight", "@every 1h30m"
func Parse(spec string) (*Schedule, error) {
	return defaultParser.Parse(spec)
}

// getField returns an Int with the bits set representing all of the times that
// the field represents or error parsing field value.  A "field" is a comma-separated
// list of "ranges".
func getField(field string, r bounds) (int64, error) {
	var bits int64
	ranges := strings.FieldsFunc(field, func(r rune) bool { return r == ',' })
	for _, expr := range ranges {
		bit, err := getRange(expr, r)
		if err != nil {
			return bits, err
		}
		bits |= bit
	}
	return bits, nil
}

// getRange returns the bits indicated by the given expression:
//
//	number | number "-" number [ "/" number ]
//
// or error parsing range.
func getRange(expr string, r bounds) (int64, error) {
	var (
		start, end, step int64
		rangeAndStep     = strings.Split(expr, "/")
		lowAndHigh       = strings.Split(rangeAndStep[0], "-")
		singleDigit      = len(lowAndHigh) == 1
		err              error
	)

	var extra int64
	if lowAndHigh[0] == "*" || lowAndHigh[0] == "?" {
		start = r.min
		end = r.max
		var ustr uint64 = starBit
		extra = int64(ustr)
	} else {
		start, err = parseIntOrName(lowAndHigh[0], r.names)
		if err != nil {
			return 0, err
		}
		switch len(lowAndHigh) {
		case 1:
			end = start
		case 2:
			end, err = parseIntOrName(lowAndHigh[1], r.names)
			if err != nil {
				return 0, err
			}
		default:
			return 0, fmt.Errorf("too many hyphens: %s", expr)
		}
	}

	switch len(rangeAndStep) {
	case 1:
		step = 1
	case 2:
		step, err = mustParseInt(rangeAndStep[1])
		if err != nil {
			return 0, err
		}

		// Special handling: "N/step" means "N-max/step".
		if singleDigit {
			end = r.max
		}
	default:
		return 0, fmt.Errorf("too many slashes: %s", expr)
	}

	if start < r.min {
		return 0, fmt.Errorf("beginning of range (%d) below minimum (%d): %s", start, r.min, expr)
	}
	if end > r.max {
		return 0, fmt.Errorf("end of range (%d) above maximum (%d): %s", end, r.max, expr)
	}
	if start > end {
		return 0, fmt.Errorf(
			"beginning of range (%d) beyond end of range (%d): %s",
			start,
			end,
			expr,
		)
	}
	if step == 0 {
		return 0, fmt.Errorf("step of range should be a positive number: %s", expr)
	}

	return getBits(start, end, step) | extra, nil
}

// parseIntOrName returns the (possibly-named) integer contained in expr.
func parseIntOrName(expr string, names map[string]int64) (int64, error) {
	if names != nil {
		if namedInt, ok := names[strings.ToLower(expr)]; ok {
			return namedInt, nil
		}
	}
	return mustParseInt(expr)
}

// mustParseInt parses the given expression as an int or returns an error.
func mustParseInt(expr string) (int64, error) {
	num, err := strconv.Atoi(expr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse int from %s: %s", expr, err)
	}
	if num < 0 {
		return 0, fmt.Errorf("negative number (%d) not allowed: %s", num, expr)
	}

	return int64(num), nil
}

// getBits sets all bits in the range [min, max], modulo the given step size.
func getBits(lowerLimit, upperLimit, step int64) int64 {
	var bits int64

	// If step is 1, use shifts.
	if step == 1 {
		return ^(toInt64(math.MaxUint64) << (upperLimit + 1)) & (toInt64(math.MaxUint64) << lowerLimit)
	}

	// Else, use a simple loop.
	for i := lowerLimit; i <= upperLimit; i += step {
		bits |= 1 << i
	}
	return bits
}

func toInt64(starBit uint64) int64 {
	return int64(starBit)
}

// all returns all bits within the given bounds.  (plus the star bit).
func all(r bounds) int64 {
	return getBits(r.min, r.max, 1) | toInt64(starBit)
}

// parseDescriptor returns a predefined schedule for the expression, or error if none matches.
func parseDescriptor(descriptor string) (*Schedule, error) {
	switch descriptor {
	case "@yearly", "@annually":
		return &Schedule{
			Second:     1 << seconds.min,
			Minute:     1 << minutes.min,
			Hour:       1 << hours.min,
			Dom:        1 << dom.min,
			Month:      1 << months.min,
			Dow:        all(dow),
			RepeatDate: 1001,
		}, nil

	case "@monthly":
		return &Schedule{
			Second:     1 << seconds.min,
			Minute:     1 << minutes.min,
			Hour:       1 << hours.min,
			Dom:        1 << dom.min,
			Month:      all(months),
			Dow:        all(dow),
			RepeatDate: 1001,
		}, nil

	case "@weekly":
		return &Schedule{
			Second:     1 << seconds.min,
			Minute:     1 << minutes.min,
			Hour:       1 << hours.min,
			Dom:        all(dom),
			Month:      all(months),
			Dow:        1 << dow.min,
			RepeatDate: 1001,
		}, nil

	case "@daily", "@midnight":
		return &Schedule{
			Second:     1 << seconds.min,
			Minute:     1 << minutes.min,
			Hour:       1 << hours.min,
			Dom:        all(dom),
			Month:      all(months),
			Dow:        all(dow),
			RepeatDate: 1001,
		}, nil

	case "@hourly":
		return &Schedule{
			Second:     1 << seconds.min,
			Minute:     1 << minutes.min,
			Hour:       all(hours),
			Dom:        all(dom),
			Month:      all(months),
			Dow:        all(dow),
			RepeatDate: 1001,
		}, nil
	}

	const every = "@every "
	if strings.HasPrefix(descriptor, every) {
		duration, err := time.ParseDuration(descriptor[len(every):])
		if err != nil {
			return nil, fmt.Errorf("failed to parse duration %s: %s", descriptor, err)
		}
		if duration < time.Hour {
			return &Schedule{
				RepeatMin: int64(math.Ceil(duration.Minutes())),
			}, nil
		} else if duration < time.Hour*24 {
			return &Schedule{
				RepeatHour: int64(math.Ceil(duration.Hours())),
			}, nil
		} else {
			return &Schedule{
				RepeatDate: int64(math.Ceil(duration.Hours() / 24)),
			}, nil
		}
	}

	return nil, fmt.Errorf("unrecognized descriptor: %s", descriptor)
}
