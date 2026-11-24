// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Maurice Snoeren
//
// This file is part of USB Backup.
//
// USB Backup is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version.
//
// USB Backup is distributed in the hope that it will be useful, but WITHOUT ANY
// WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS FOR A
// PARTICULAR PURPOSE. See the GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along with
// USB Backup. If not, see <https://www.gnu.org/licenses/>.

package config

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/macsnoeren/backup-usb/internal/week"
)

// Keys returns all setting keys, sorted.
func Keys() []string {
	t := reflect.TypeOf(Config{})
	out := make([]string, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		out = append(out, jsonName(t.Field(i)))
	}
	sort.Strings(out)
	return out
}

func jsonName(f reflect.StructField) string {
	name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
	return name
}

func field(c *Config, key string) (reflect.Value, bool) {
	v := reflect.ValueOf(c).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		if jsonName(t.Field(i)) == key {
			return v.Field(i), true
		}
	}
	return reflect.Value{}, false
}

// Get returns a setting as JSON.
func (c *Config) Get(key string) (string, error) {
	f, ok := field(c, key)
	if !ok {
		return "", fmt.Errorf("unknown setting %q", key)
	}
	data, err := json.Marshal(f.Interface())
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// Set changes one setting from its text form: a string, a number, true or
// false, a list as a JSON array or comma separated, or JSON for sources.
// It returns the old and new value as JSON. The caller validates the
// result. A folder pattern that does not fit a new schedule becomes the
// default pattern of that schedule.
func (c *Config) Set(key, value string) (oldV, newV string, err error) {
	f, ok := field(c, key)
	if !ok {
		return "", "", fmt.Errorf("unknown setting %q", key)
	}
	oldV, _ = c.Get(key)
	switch f.Kind() {
	case reflect.String:
		f.SetString(value)
		if key == "schedule" && c.Schedule.Valid() {
			if _, err := week.ParsePattern(c.FolderPattern, c.Schedule); err != nil {
				c.FolderPattern = week.DefaultPattern(c.Schedule)
			}
		}
	case reflect.Int:
		n, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return "", "", fmt.Errorf("%s: %q is not a whole number", key, value)
		}
		f.SetInt(int64(n))
	case reflect.Bool:
		b, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return "", "", fmt.Errorf("%s: %q is not true or false", key, value)
		}
		f.SetBool(b)
	case reflect.Slice:
		ptr := reflect.New(f.Type())
		v := strings.TrimSpace(value)
		if strings.HasPrefix(v, "[") {
			if err := json.Unmarshal([]byte(v), ptr.Interface()); err != nil {
				return "", "", fmt.Errorf("%s: %w", key, err)
			}
		} else if f.Type().Elem().Kind() == reflect.String {
			var parts []string
			for _, p := range strings.Split(v, ",") {
				if p = strings.TrimSpace(p); p != "" {
					parts = append(parts, p)
				}
			}
			ptr.Elem().Set(reflect.ValueOf(parts))
		} else {
			return "", "", fmt.Errorf("%s: give a JSON array", key)
		}
		f.Set(ptr.Elem())
	default:
		return "", "", fmt.Errorf("%s cannot be set this way", key)
	}
	newV, _ = c.Get(key)
	return oldV, newV, nil
}
