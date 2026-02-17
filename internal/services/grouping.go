// Package services provides service processing and grouping functionality for the Trala dashboard.
// This file contains the grouping algorithm that organizes services by common tags.
package services

import (
	"math"
	"sort"

	"server/internal/config"
	"server/internal/models"
)

// CalculateGroups implements the grouping algorithm for services.
// It assigns services to groups based on common tags, respecting any pre-assigned groups.
func CalculateGroups(services []models.Service) []models.Service {
	if !config.GetGroupingEnabled() {
		for i := range services {
			services[i].Group = ""
		}
		return services
	}

	// First, assign from overrides by checking if service.Group is already set
	remainingIndices := make([]int, 0, len(services))
	for i, s := range services {
		// Check if the service already has a group set (from override)
		if s.Group != "" {
			// Service already has a group assigned, keep it
			services[i].Group = s.Group
		} else {
			remainingIndices = append(remainingIndices, i)
		}
	}

	// Now, for remaining, do the grouping
	if len(remainingIndices) == 0 {
		return services
	}

	// Get remaining services
	remaining := make([]models.Service, len(remainingIndices))
	for i, idx := range remainingIndices {
		remaining[i] = services[idx]
	}

	// Preprocessing: calculate tag frequencies
	tagCount, _ := calculateTagFrequencies(remaining)

	// Filter tags
	validTags := filterValidTags(remaining, tagCount)

	targetSize := math.Sqrt(float64(len(remaining)))

	for len(remaining) > 0 && len(validTags) > 0 {
		bestTag := selectBestTag(validTags, tagCount, targetSize)
		if bestTag == "" {
			break
		}
		groupName := bestTag
		remainingIndices = assignGroupToServices(services, remainingIndices, bestTag, groupName)

		// Update remaining
		remaining = make([]models.Service, len(remainingIndices))
		for i, idx := range remainingIndices {
			remaining[i] = services[idx]
		}

		// Remove bestTag from validTags
		newValidTags := make([]string, 0, len(validTags))
		for _, t := range validTags {
			if t != bestTag {
				newValidTags = append(newValidTags, t)
			}
		}
		validTags = newValidTags

		// Update tagCount
		tagCount, _ = calculateTagFrequencies(remaining)
	}

	return services
}

// calculateTagFrequencies calculates the frequency of each tag and the number of tags per service.
// It returns tagCount (map of tag to count) and serviceTagCount (map of service name to tag count).
func calculateTagFrequencies(remaining []models.Service) (map[string]int, map[string]int) {
	tagCount := make(map[string]int)
	serviceTagCount := make(map[string]int)
	for _, s := range remaining {
		serviceTagCount[s.Name] = len(s.Tags)
		for _, tag := range s.Tags {
			tagCount[tag]++
		}
	}
	return tagCount, serviceTagCount
}

// filterValidTags filters tags based on frequency thresholds and ensures meaningful grouping.
// Tags that are too common (above frequency threshold) are excluded unless they meet minimum services per group.
// Single-occurrence tags are only included if there's a service with exactly that one tag.
func filterValidTags(remaining []models.Service, tagCount map[string]int) []string {
	validTags := make([]string, 0)
	total := len(remaining)
	threshold := int(config.GetTagFrequencyThreshold() * float64(total))
	minServicesPerGroup := config.GetMinServicesPerGroup()

	for tag, count := range tagCount {
		// Case 1: Skip tags that are too common (above frequency threshold) and don't meet minimum services
		if count > threshold && count < minServicesPerGroup {
			continue
		}

		// Case 2: Handle single-occurrence tags
		if count == 1 && minServicesPerGroup > 1 {
			// Only include single tags if there's a service with exactly that one tag
			// If minServicesPerGroup == 1, single tags are included by default
			for _, s := range remaining {
				if len(s.Tags) == 1 && s.Tags[0] == tag {
					validTags = append(validTags, tag)
					break
				}
			}
		} else if count >= minServicesPerGroup {
			// Case 3: Include tags that meet the minimum services requirement
			validTags = append(validTags, tag)
		}
	}

	sort.Strings(validTags)
	return validTags
}

// selectBestTag selects the best tag from validTags based on group size proximity to targetSize.
// It calculates a score where smaller groups closer to targetSize are preferred.
func selectBestTag(validTags []string, tagCount map[string]int, targetSize float64) string {
	bestTag := ""
	bestScore := -1e9
	for _, tag := range validTags {
		groupSize := tagCount[tag]
		var score float64
		// Score based on how CLOSE the group size is to target (smaller distance = better)
		// Use negative distance so higher score = better match
		score = -math.Abs(float64(groupSize) - targetSize)
		if score > bestScore {
			bestScore = score
			bestTag = tag
		}
	}
	return bestTag
}

// assignGroupToServices assigns the groupName to services that have the bestTag and returns the updated remainingIndices.
func assignGroupToServices(services []models.Service, remainingIndices []int, bestTag, groupName string) []int {
	newRemainingIndices := make([]int, 0, len(remainingIndices))
	for _, idx := range remainingIndices {
		s := &services[idx]
		hasTag := false
		for _, t := range s.Tags {
			if t == bestTag {
				hasTag = true
				break
			}
		}
		if hasTag {
			s.Group = groupName
		} else {
			newRemainingIndices = append(newRemainingIndices, idx)
		}
	}
	return newRemainingIndices
}
