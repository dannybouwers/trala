package config

import (
	"fmt"
	"log"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"
)

// envVarAliases maps canonical environment variable names to their deprecated
// aliases. The canonical name always wins; the alias is only consulted when
// the canonical name is unset.
var envVarAliases = map[string]string{
	"GROUPING_COLUMNS": "GROUPED_COLUMNS",
}

var groupedColumnsAliasWarnOnce sync.Once

// envApplyValidator enforces per-field numeric/oneof constraints at env-apply
// time. It is separate from the validator used by Validate() so the tag-name
// registration there stays untouched.
var envApplyValidator = validator.New()

// envVarNameForPath returns the environment variable name for a YAML path
// relative to `environment.`. Indexed instance segments (`instances[i]`) are
// rewritten to 1-based `instances_<n>` segments first, so
// `traefik.instances[1].basic_auth.username` maps to
// `TRAEFIK_INSTANCES_2_BASIC_AUTH_USERNAME`.
func envVarNameForPath(yamlPath string) string {
	rewritten := rewriteInstanceSegments(yamlPath)
	return EnvironmentEnvVar("environment." + rewritten)
}

// rewriteInstanceSegments converts every `instances[i]` path segment into
// `instances_<i+1>`. Segments without an index are left unchanged.
func rewriteInstanceSegments(path string) string {
	parts := strings.Split(path, ".")
	for i, part := range parts {
		if !strings.HasPrefix(part, "instances[") || !strings.HasSuffix(part, "]") {
			continue
		}
		idxStr := strings.TrimSuffix(strings.TrimPrefix(part, "instances["), "]")
		idx, err := strconv.Atoi(idxStr)
		if err != nil || idx < 0 {
			continue
		}
		parts[i] = fmt.Sprintf("instances_%d", idx+1)
	}
	return strings.Join(parts, ".")
}

// lookupEnv resolves an environment variable by its canonical name. An empty
// value is treated as unset. When the canonical name is unset, its deprecated
// alias (if any) is consulted and a deprecation warning is logged once.
func lookupEnv(canonical string) (string, bool) {
	if v := os.Getenv(canonical); v != "" {
		return v, true
	}
	alias, ok := envVarAliases[canonical]
	if !ok {
		return "", false
	}
	v := os.Getenv(alias)
	if v == "" {
		return "", false
	}
	groupedColumnsAliasWarnOnce.Do(func() {
		log.Printf("WARNING: %s is deprecated and will be removed in a future release. Use %s instead.", alias, canonical)
	})
	return v, true
}

// applyEnvironmentOverrides applies every `environment.*` option from
// environment variables onto the configuration, using names derived from the
// YAML field paths. Values that fail to parse or violate the field's numeric
// or oneof constraints are ignored with a warning, keeping the previous value.
func applyEnvironmentOverrides(c *TralaConfiguration) {
	debug := c.Environment.LogLevel == "debug" || os.Getenv("LOG_LEVEL") == "debug"
	root := reflect.ValueOf(c).Elem()
	applyEnvToStruct(root.FieldByName("Environment"), "", debug)
}

// applyEnvToStruct recursively walks a configuration struct and applies
// environment overrides to every scalar leaf. Paths are built from yaml tags.
// The Traefik subtree is delegated to applyTraefikEnvOverrides because its
// instance handling cannot be expressed as a plain path mapping.
func applyEnvToStruct(v reflect.Value, prefix string, debug bool) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}
		yamlTag := strings.SplitN(field.Tag.Get("yaml"), ",", 2)[0]
		if yamlTag == "" || yamlTag == "-" {
			continue
		}
		path := yamlTag
		if prefix != "" {
			path = prefix + "." + yamlTag
		}
		fv := v.Field(i)
		if fv.Kind() == reflect.Struct {
			if fv.Type() == reflect.TypeOf(TraefikConfig{}) {
				applyTraefikEnvOverrides(fv.Addr().Interface().(*TraefikConfig), debug)
				continue
			}
			applyEnvToStruct(fv, path, debug)
			continue
		}
		setFieldFromEnv(fv, field.Tag.Get("validate"), envVarNameForPath(path), path, debug)
	}
}

// setFieldFromEnv applies a single environment variable to a field. Unset or
// empty variables are skipped. Values that fail to parse for the field kind,
// or that violate the field's enforceable validate constraints, are ignored
// with a warning and the previous value is kept. Reports whether a value was
// applied.
func setFieldFromEnv(field reflect.Value, validateTag, envName, path string, debug bool) bool {
	parsed, raw, ok := parseEnvValue(field, validateTag, envName, path)
	if !ok {
		return false
	}
	applyParsed(field, parsed, envName, path, raw, debug)
	return true
}

// parseEnvValue resolves the environment variable for a field and parses it
// against the field's kind and enforceable validate constraints, without
// writing anything. Returns ok=false when the variable is unset, unparsable,
// or constraint-violating; failures are logged.
func parseEnvValue(field reflect.Value, validateTag, envName, path string) (reflect.Value, string, bool) {
	raw, ok := lookupEnv(envName)
	if !ok {
		return reflect.Value{}, "", false
	}

	var parsed reflect.Value
	switch field.Kind() {
	case reflect.String:
		parsed = reflect.ValueOf(raw)
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			log.Printf("Warning: Invalid %s '%s' for %s, keeping previous value", envName, raw, path)
			return reflect.Value{}, "", false
		}
		parsed = reflect.ValueOf(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, field.Type().Bits())
		if err != nil {
			log.Printf("Warning: Invalid %s '%s' for %s, keeping previous value", envName, raw, path)
			return reflect.Value{}, "", false
		}
		parsed = reflect.ValueOf(n).Convert(field.Type())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(raw, 10, field.Type().Bits())
		if err != nil {
			log.Printf("Warning: Invalid %s '%s' for %s, keeping previous value", envName, raw, path)
			return reflect.Value{}, "", false
		}
		parsed = reflect.ValueOf(n).Convert(field.Type())
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(raw, field.Type().Bits())
		if err != nil {
			log.Printf("Warning: Invalid %s '%s' for %s, keeping previous value", envName, raw, path)
			return reflect.Value{}, "", false
		}
		parsed = reflect.ValueOf(f).Convert(field.Type())
	default:
		return reflect.Value{}, "", false
	}

	if constraints := enforceableConstraints(validateTag); constraints != "" {
		if err := envApplyValidator.Var(parsed.Interface(), constraints); err != nil {
			log.Printf("Warning: Invalid %s '%s' for %s, keeping previous value", envName, raw, path)
			return reflect.Value{}, "", false
		}
	}
	return parsed, raw, true
}

// applyParsed writes a parsed environment value to a field and optionally logs
// the applied override.
func applyParsed(field reflect.Value, parsed reflect.Value, envName, path, raw string, debug bool) {
	field.Set(parsed)
	if debug {
		logRaw := raw
		if strings.Contains(path, "password") || strings.Contains(path, "password_file") {
			logRaw = "***REDACTED***"
		}
		log.Printf("DEBUG: env override applied: %s -> %s = %s", envName, path, logRaw)
	}
}
		log.Printf("DEBUG: env override applied: %s -> %s = %s", envName, path, logRaw)
	}
}

// enforceableVarTags are the validate tags enforced when applying environment
// values. required/url/dive are excluded on purpose: they still surface as
// hard load errors through Validate().
var enforceableVarTags = map[string]bool{
	"gte":   true,
	"gt":    true,
	"lte":   true,
	"lt":    true,
	"oneof": true,
}

// enforceableConstraints filters a validate tag down to the constraints that
// are enforced at env-apply time. Returns "" when nothing remains.
func enforceableConstraints(tag string) string {
	if tag == "" {
		return ""
	}
	parts := strings.Split(tag, ",")
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		name := strings.SplitN(part, "=", 2)[0]
		if enforceableVarTags[name] {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, ",")
}

// instanceEnvField pairs an environment variable suffix with the dotted yaml
// path of the matching TraefikInstanceConfig field.
type instanceEnvField struct {
	suffix string
	path   string
}

// instanceEnvFields lists, in fixed order, the environment variable suffixes
// recognized for both indexed (TRAEFIK_INSTANCES_<n>_<SUFFIX>) and legacy
// (TRAEFIK_<SUFFIX>) variables, derived from the yaml tags of
// TraefikInstanceConfig (and nested structs), so new instance fields are
// picked up automatically. Fixed order keeps log output deterministic.
var instanceEnvFields = buildInstanceEnvFields()

func buildInstanceEnvFields() []instanceEnvField {
	var fields []instanceEnvField
	var walk func(t reflect.Type, prefix string)
	walk = func(t reflect.Type, prefix string) {
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if field.PkgPath != "" {
				continue
			}
			yamlTag := strings.SplitN(field.Tag.Get("yaml"), ",", 2)[0]
			if yamlTag == "" || yamlTag == "-" {
				continue
			}
			path := yamlTag
			if prefix != "" {
				path = prefix + "." + yamlTag
			}
			if field.Type.Kind() == reflect.Struct {
				walk(field.Type, path)
				continue
			}
			fields = append(fields, instanceEnvField{
				suffix: strings.ToUpper(strings.ReplaceAll(path, ".", "_")),
				path:   path,
			})
		}
	}
	walk(reflect.TypeOf(TraefikInstanceConfig{}), "")
	return fields
}

// applyTraefikEnvOverrides applies Traefik environment variables. Indexed
// TRAEFIK_INSTANCES_<n>_<SUFFIX> variables (1-based) are processed first and
// merge by index into the instances slice, switching the config to
// instance-format. Legacy TRAEFIK_<SUFFIX> variables then apply to the first
// instance in single-instance mode, or are ignored with a warning in
// multi-instance mode.
func applyTraefikEnvOverrides(t *TraefikConfig, debug bool) {
	appliedIndexed := false
	for _, entry := range collectIndexedInstanceEnvVars() {
		path, known := knownInstanceSuffix(entry.suffix)
		if !known {
			log.Printf("Warning: Unknown TRAEFIK_INSTANCES_%d_%s, ignoring", entry.index, entry.suffix)
			continue
		}
		fullPath := fmt.Sprintf("traefik.instances[%d].%s", entry.index, path)
		probe := TraefikInstanceConfig{}
		probeField, validateTag, ok := instanceFieldByPath(&probe, path)
		if !ok {
			log.Printf("Warning: Unknown TRAEFIK_INSTANCES_%d_%s, ignoring", entry.index, entry.suffix)
			continue
		}
		// Parse before growing the slice so unusable values never create
		// empty instances as a structural side effect.
		parsed, raw, ok := parseEnvValue(probeField, validateTag, entry.key, fullPath)
		if !ok {
			continue
		}
		for len(t.Instances) < entry.index {
			t.Instances = append(t.Instances, TraefikInstanceConfig{})
		}
		target, _, ok := instanceFieldByPath(&t.Instances[entry.index-1], path)
		if !ok {
			continue
		}
		applyParsed(target, parsed, entry.key, fullPath, raw, debug)
		appliedIndexed = true
	}
	if appliedIndexed {
		t.IsMulti = true
	}

	if t.IsMulti {
		for _, key := range legacyTraefikEnvKeys() {
			if os.Getenv(key) != "" {
				log.Printf("WARNING: Multi-instance mode detected: %s and related env vars are ignored. Use configuration file instead.", key)
				break
			}
		}
		return
	}

	if len(t.Instances) == 0 {
		t.Instances = append(t.Instances, TraefikInstanceConfig{})
	}
	inst := &t.Instances[0]
	for _, envField := range instanceEnvFields {
		field, validateTag, ok := instanceFieldByPath(inst, envField.path)
		if !ok {
			continue
		}
		setFieldFromEnv(field, validateTag, "TRAEFIK_"+envField.suffix, "traefik.instances[0]."+envField.path, debug)
	}
}

// legacyTraefikEnvKeys returns the legacy single-instance environment variable
// names that are ignored in multi-instance mode. Mirrors the historical key
// list, which predates indexed instance variables.
func legacyTraefikEnvKeys() []string {
	keys := make([]string, 0, len(instanceEnvFields))
	for _, envField := range instanceEnvFields {
		if envField.suffix == "NAME" {
			continue
		}
		keys = append(keys, "TRAEFIK_"+envField.suffix)
	}
	return keys
}

// indexedInstanceEnvVar is one matched TRAEFIK_INSTANCES_<n>_<SUFFIX> variable.
type indexedInstanceEnvVar struct {
	key    string
	index  int
	suffix string
}

// collectIndexedInstanceEnvVars scans the environment for
// TRAEFIK_INSTANCES_<n>_<SUFFIX> variables and returns them sorted by index
// and suffix for deterministic processing. Entries with an unusable index
// (non-numeric or < 1) are warned about and dropped.
func collectIndexedInstanceEnvVars() []indexedInstanceEnvVar {
	const prefix = "TRAEFIK_INSTANCES_"
	var matched []indexedInstanceEnvVar
	for _, kv := range os.Environ() {
		key, value, ok := strings.Cut(kv, "=")
		if !ok || !strings.HasPrefix(key, prefix) || value == "" {
			continue
		}
		rest := strings.TrimPrefix(key, prefix)
		underscore := strings.Index(rest, "_")
		if underscore <= 0 {
			log.Printf("Warning: Invalid TRAEFIK_INSTANCES variable %s, ignoring", key)
			continue
		}
		idx, err := strconv.Atoi(rest[:underscore])
		if err != nil || idx < 1 {
			log.Printf("Warning: Invalid TRAEFIK_INSTANCES index in %s, ignoring", key)
			continue
		}
		matched = append(matched, indexedInstanceEnvVar{
			key:    key,
			index:  idx,
			suffix: rest[underscore+1:],
		})
	}
	sort.Slice(matched, func(i, j int) bool {
		if matched[i].index != matched[j].index {
			return matched[i].index < matched[j].index
		}
		return matched[i].suffix < matched[j].suffix
	})
	return matched
}

// knownInstanceSuffix reports whether a suffix is recognized and returns the
// corresponding dotted yaml path inside TraefikInstanceConfig.
func knownInstanceSuffix(suffix string) (string, bool) {
	for _, envField := range instanceEnvFields {
		if envField.suffix == suffix {
			return envField.path, true
		}
	}
	return "", false
}

// instanceFieldByPath navigates a TraefikInstanceConfig value along a dotted
// yaml path and returns the addressable leaf field together with its validate
// tag.
func instanceFieldByPath(inst *TraefikInstanceConfig, path string) (reflect.Value, string, bool) {
	cur := reflect.ValueOf(inst).Elem()
	var tag string
	for _, segment := range strings.Split(path, ".") {
		if cur.Kind() != reflect.Struct {
			return reflect.Value{}, "", false
		}
		next, field, found := structFieldByYAMLTag(cur, segment)
		if !found {
			return reflect.Value{}, "", false
		}
		cur = next
		tag = field.Tag.Get("validate")
	}
	return cur, tag, true
}

// structFieldByYAMLTag finds the struct field whose yaml tag matches name and
// returns its value and StructField metadata.
func structFieldByYAMLTag(v reflect.Value, name string) (reflect.Value, reflect.StructField, bool) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}
		yamlTag := strings.SplitN(field.Tag.Get("yaml"), ",", 2)[0]
		if yamlTag == name {
			return v.Field(i), field, true
		}
	}
	return reflect.Value{}, reflect.StructField{}, false
}
