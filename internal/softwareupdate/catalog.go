// Package softwareupdate models Apple's Software Update catalog independently
// of the CLI, planner, and privilege-escalation machinery.
package softwareupdate

import (
	"bufio"
	"fmt"
	"regexp"
	"strings"
)

type Update struct {
	Label   string
	Title   string
	Version string
	Restart bool
}

type Request struct {
	Name             string
	Match            string // title (exact), prefix (title prefix), or label (exact)
	Receipt          string
	Latest           bool
	Restart          bool
	CommandLineTools bool
}

var metadata = regexp.MustCompile(`^Title: (.+), Version: ([^,]+), Size: .+$`)
var receiptName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func (r Request) Validate() error {
	if strings.TrimSpace(r.Name) != r.Name || r.Name == "" || strings.HasPrefix(r.Name, "-") || strings.ContainsAny(r.Name, "\r\n\x00") {
		return fmt.Errorf("software update name must be nonempty, single-line, and not an option")
	}
	if r.Match != "title" && r.Match != "prefix" && r.Match != "label" {
		return fmt.Errorf("match must be title, prefix, or label")
	}
	if r.Receipt != "" && !receiptName.MatchString(r.Receipt) {
		return fmt.Errorf("invalid package receipt ID %q", r.Receipt)
	}
	if !r.Latest && r.Receipt == "" {
		return fmt.Errorf("state present requires a package receipt ID")
	}
	return nil
}

// ParseCatalog rejects unexpected or incomplete output instead of treating a
// failed scan as proof that the machine is current.
func ParseCatalog(output string) ([]Update, error) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	var updates []Update
	var pending string
	seen := make(map[string]bool)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if label, ok := strings.CutPrefix(line, "* Label: "); ok {
			if pending != "" || label == "" || strings.HasPrefix(label, "-") || strings.ContainsRune(label, 0) || seen[label] {
				return nil, fmt.Errorf("invalid or duplicate Software Update label %q", label)
			}
			pending = label
			seen[label] = true
			continue
		}
		if strings.HasPrefix(line, "Title:") {
			fields := metadata.FindStringSubmatch(line)
			if pending == "" || fields == nil {
				return nil, fmt.Errorf("incomplete Software Update metadata: %s", line)
			}
			restart := false
			if _, actionText, exists := strings.Cut(line, ", Action: "); exists {
				actionText = strings.TrimSpace(strings.TrimSuffix(actionText, ","))
				if actionText != "restart" && actionText != "shut down" {
					return nil, fmt.Errorf("unrecognized update action %q", actionText)
				}
				restart = true
			}
			updates = append(updates, Update{Label: pending, Title: fields[1], Version: strings.TrimSpace(fields[2]), Restart: restart})
			pending = ""
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read Software Update catalog: %w", err)
	}
	if pending != "" {
		return nil, fmt.Errorf("missing metadata for update %s", pending)
	}
	if len(updates) == 0 && !strings.Contains(output, "No new software available") {
		return nil, fmt.Errorf("softwareupdate returned no recognizable catalog: %s", strings.TrimSpace(output))
	}
	return updates, nil
}

func (r Request) Select(updates []Update) (*Update, error) {
	var selected *Update
	for _, update := range updates {
		matches := (r.Match == "title" && update.Title == r.Name) || (r.Match == "prefix" && strings.HasPrefix(update.Title, r.Name)) || (r.Match == "label" && update.Label == r.Name)
		if !matches {
			continue
		}
		if selected == nil {
			selected = &update
			continue
		}
		order, err := compareVersions(update.Version, selected.Version)
		if err != nil {
			return nil, err
		}
		if order == 0 && selected.Label != update.Label {
			return nil, fmt.Errorf("multiple updates match %q at version %s; select an exact label", r.Name, update.Version)
		}
		if order > 0 {
			selected = &update
		}
	}
	return selected, nil
}

func compareVersions(a, b string) (int, error) {
	left, right := strings.Split(a, "."), strings.Split(b, ".")
	for _, version := range [][]string{left, right} {
		for _, part := range version {
			if part == "" || strings.IndexFunc(part, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
				return 0, fmt.Errorf("cannot order update versions %q and %q; select an exact label", a, b)
			}
		}
	}
	for i := 0; i < max(len(left), len(right)); i++ {
		x, y := "", ""
		if i < len(left) {
			x = strings.TrimLeft(left[i], "0")
		}
		if i < len(right) {
			y = strings.TrimLeft(right[i], "0")
		}
		if len(x) < len(y) {
			return -1, nil
		}
		if len(x) > len(y) {
			return 1, nil
		}
		if order := strings.Compare(x, y); order != 0 {
			return order, nil
		}
	}
	return 0, nil
}
