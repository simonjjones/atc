package config

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/concourse/atc"
)

type InvalidConfigError struct {
	GroupsErr    error
	ResourcesErr error
	JobsErr      error
	PluginsErr   error
}

func (err InvalidConfigError) Error() string {
	errorMsgs := []string{"invalid configuration:"}

	if err.GroupsErr != nil {
		errorMsgs = append(errorMsgs, indent(fmt.Sprintf("invalid groups:\n%s\n", indent(err.GroupsErr.Error()))))
	}

	if err.ResourcesErr != nil {
		errorMsgs = append(errorMsgs, indent(fmt.Sprintf("invalid resources:\n%s\n", indent(err.ResourcesErr.Error()))))
	}

	if err.JobsErr != nil {
		errorMsgs = append(errorMsgs, indent(fmt.Sprintf("invalid jobs:\n%s\n", indent(err.JobsErr.Error()))))
	}

	return strings.Join(errorMsgs, "\n")
}

func indent(msgs string) string {
	lines := strings.Split(msgs, "\n")
	indented := make([]string, len(lines))

	for i, l := range lines {
		indented[i] = "\t" + l
	}

	return strings.Join(indented, "\n")
}

func ValidateConfig(c atc.Config) error {
	groupsErr := validateGroups(c)
	resourcesErr := validateResources(c)
	jobsErr := validateJobs(c)
	pluginsErr := validatePlugins(c)

	if groupsErr == nil && resourcesErr == nil && jobsErr == nil && pluginsErr == nil {
		return nil
	}

	return InvalidConfigError{
		GroupsErr:    groupsErr,
		ResourcesErr: resourcesErr,
		JobsErr:      jobsErr,
		PluginsErr:   pluginsErr,
	}
}

func validateGroups(c atc.Config) error {
	errorMessages := []string{}

	for _, group := range c.Groups {
		for _, job := range group.Jobs {
			_, exists := c.Jobs.Lookup(job)
			if !exists {
				errorMessages = append(errorMessages,
					fmt.Sprintf("group '%s' has unknown job '%s'", group.Name, job))
			}
		}

		for _, resource := range group.Resources {
			_, exists := c.Resources.Lookup(resource)
			if !exists {
				errorMessages = append(errorMessages,
					fmt.Sprintf("group '%s' has unknown resource '%s'", group.Name, resource))
			}
		}
	}

	return compositeErr(errorMessages)
}

func validateResources(c atc.Config) error {
	errorMessages := []string{}

	names := map[string]int{}

	for i, resource := range c.Resources {
		var identifier string
		if resource.Name == "" {
			identifier = fmt.Sprintf("resources[%d]", i)
		} else {
			identifier = fmt.Sprintf("resources.%s", resource.Name)
		}

		if other, exists := names[resource.Name]; exists {
			errorMessages = append(errorMessages,
				fmt.Sprintf(
					"resources[%d] and resources[%d] have the same name ('%s')",
					other, i, resource.Name))
		} else if resource.Name != "" {
			names[resource.Name] = i
		}

		if resource.Name == "" {
			errorMessages = append(errorMessages, identifier+" has no name")
		}

		if resource.Type == "" {
			errorMessages = append(errorMessages, identifier+" has no type")
		}
	}

	return compositeErr(errorMessages)
}

func validateJobs(c atc.Config) error {
	errorMessages := []string{}

	names := map[string]int{}

	for i, job := range c.Jobs {
		var identifier string
		if job.Name == "" {
			identifier = fmt.Sprintf("jobs[%d]", i)
		} else {
			identifier = fmt.Sprintf("jobs.%s", job.Name)
		}

		if other, exists := names[job.Name]; exists {
			errorMessages = append(errorMessages,
				fmt.Sprintf(
					"jobs[%d] and jobs[%d] have the same name ('%s')",
					other, i, job.Name))
		} else if job.Name != "" {
			names[job.Name] = i
		}

		if job.Name == "" {
			errorMessages = append(errorMessages, identifier+" has no name")
		}

		if job.Plan != nil && (job.TaskConfig != nil || len(job.TaskConfigPath) > 0 || len(job.InputConfigs) > 0 || len(job.OutputConfigs) > 0) {
			errorMessages = append(errorMessages, identifier+" has both a plan and inputs/outputs/build config specified")
		}

		errorMessages = append(errorMessages, validateConditionals(identifier+".plan", job.Plan)...)
		errorMessages = append(errorMessages, validatePlan(c, identifier+".plan", atc.PlanConfig{Do: &job.Plan})...)
		errorMessages = append(errorMessages, validateInputOutputConfig(c, job, identifier)...)
	}

	return compositeErr(errorMessages)
}

func validateConditionals(identifier string, planSequence atc.PlanSequence) []string {
	hasConditionals := hasConditionals(planSequence)
	hasHooks := hasHooks(planSequence)

	if hasConditionals && hasHooks {
		return []string{
			fmt.Sprintf("%s has both conditions and hooks specified", identifier),
		}
	}

	return []string{}
}

func hasConditionals(planSequence atc.PlanSequence) bool {
	return doesAnyStepMatch(planSequence, func(step atc.PlanConfig) bool {
		return step.Conditions != nil
	})
}

func hasHooks(planSequence atc.PlanSequence) bool {
	return doesAnyStepMatch(planSequence, func(step atc.PlanConfig) bool {
		return step.Failure != nil || step.Ensure != nil || step.Success != nil
	})
}

func doesAnyStepMatch(planSequence atc.PlanSequence, predicate func(step atc.PlanConfig) bool) bool {
	for _, planStep := range planSequence {
		if planStep.Aggregate != nil {
			if doesAnyStepMatch(*planStep.Aggregate, predicate) {
				return true
			}
		}

		if planStep.Do != nil {
			if doesAnyStepMatch(*planStep.Do, predicate) {
				return true
			}
		}

		if predicate(planStep) {
			return true
		}
	}

	return false
}

type foundTypes struct {
	identifier string
	found      map[string]bool
}

func (ft *foundTypes) Find(name string) {
	ft.found[name] = true
}

func (ft foundTypes) IsValid() (bool, string) {
	if len(ft.found) == 0 {
		return false, ft.identifier + " has no action specified"
	}

	if len(ft.found) > 1 {
		types := make([]string, 0, len(ft.found))

		for typee, _ := range ft.found {
			types = append(types, typee)
		}

		sort.Strings(types)

		return false, fmt.Sprintf("%s has multiple actions specified (%s)", ft.identifier, strings.Join(types, ", "))
	}

	return true, ""
}

func validatePlan(c atc.Config, identifier string, plan atc.PlanConfig) []string {
	foundTypes := foundTypes{
		identifier: identifier,
		found:      make(map[string]bool),
	}

	if plan.Get != "" {
		foundTypes.Find("get")
	}

	if plan.Put != "" {
		foundTypes.Find("put")
	}

	if plan.Task != "" {
		foundTypes.Find("task")
	}

	if plan.Do != nil {
		foundTypes.Find("do")
	}

	if plan.Aggregate != nil {
		foundTypes.Find("aggregate")
	}

	if plan.Try != nil {
		foundTypes.Find("try")
	}

	if valid, message := foundTypes.IsValid(); !valid {
		return []string{message}
	}

	errorMessages := []string{}

	switch {
	case plan.Do != nil:
		for i, plan := range *plan.Do {
			subIdentifier := fmt.Sprintf("%s[%d]", identifier, i)
			errorMessages = append(errorMessages, validatePlan(c, subIdentifier, plan)...)
		}

	case plan.Aggregate != nil:
		for i, plan := range *plan.Aggregate {
			subIdentifier := fmt.Sprintf("%s.aggregate[%d]", identifier, i)
			errorMessages = append(errorMessages, validatePlan(c, subIdentifier, plan)...)
		}

	case plan.Get != "":
		subIdentifier := fmt.Sprintf("%s.get.%s", identifier, plan.Get)

		errorMessages = append(errorMessages, validateInapplicableFields(
			[]string{"privileged", "config", "file"},
			plan, subIdentifier)...,
		)

		if plan.Resource != "" {
			_, found := c.Resources.Lookup(plan.Resource)
			if !found {
				errorMessages = append(
					errorMessages,
					fmt.Sprintf(
						"%s refers to a resource that does not exist ('%s')",
						subIdentifier,
						plan.Resource,
					),
				)
			}
		} else {
			_, found := c.Resources.Lookup(plan.Get)
			if !found {
				errorMessages = append(
					errorMessages,
					fmt.Sprintf(
						"%s refers to a resource that does not exist",
						subIdentifier,
					),
				)
			}
		}

		for _, job := range plan.Passed {
			jobConfig, found := c.Jobs.Lookup(job)
			if !found {
				errorMessages = append(
					errorMessages,
					fmt.Sprintf(
						"%s.passed references an unknown job ('%s')",
						subIdentifier,
						job,
					),
				)
			} else {
				foundResource := false
				for _, jobInput := range jobConfig.Inputs() {
					if jobInput.Resource == plan.ResourceName() {
						foundResource = true
						break
					}
				}
				for _, jobOutput := range jobConfig.Outputs() {
					if jobOutput.Resource == plan.ResourceName() {
						foundResource = true
						break
					}
				}
				if !foundResource {
					errorMessages = append(
						errorMessages,
						fmt.Sprintf(
							"%s.passed references a job ('%s') which doesn't interact with the resource ('%s')",
							subIdentifier,
							job,
							plan.Get,
						),
					)
				}
			}
		}

	case plan.Put != "":
		subIdentifier := fmt.Sprintf("%s.put.%s", identifier, plan.Put)

		errorMessages = append(errorMessages, validateInapplicableFields(
			[]string{"passed", "trigger", "privileged", "config", "file"},
			plan, subIdentifier)...,
		)

		if plan.Resource != "" {
			_, found := c.Resources.Lookup(plan.Resource)
			if !found {
				errorMessages = append(
					errorMessages,
					fmt.Sprintf(
						"%s refers to a resource that does not exist ('%s')",
						subIdentifier,
						plan.Resource,
					),
				)
			}
		} else {
			_, found := c.Resources.Lookup(plan.Put)
			if !found {
				errorMessages = append(
					errorMessages,
					fmt.Sprintf(
						"%s refers to a resource that does not exist",
						subIdentifier,
					),
				)
			}
		}

	case plan.Task != "":
		subIdentifier := fmt.Sprintf("%s.task.%s", identifier, plan.Task)

		if plan.TaskConfig == nil && plan.TaskConfigPath == "" {
			errorMessages = append(errorMessages, subIdentifier+" does not specify any task configuration")
		}

		errorMessages = append(errorMessages, validateInapplicableFields(
			[]string{"resource", "passed", "trigger"},
			plan, subIdentifier)...,
		)

		if plan.Params != nil {
			errorMessages = append(errorMessages, subIdentifier+" specifies params, which should be config.params")
		}

	case plan.Try != nil:
		subIdentifier := fmt.Sprintf("%s.try", identifier)
		errorMessages = append(errorMessages, validatePlan(c, subIdentifier, *plan.Try)...)
	}

	if plan.Ensure != nil {
		subIdentifier := fmt.Sprintf("%s.ensure", identifier)
		errorMessages = append(errorMessages, validatePlan(c, subIdentifier, *plan.Ensure)...)
	}

	if plan.Success != nil {
		subIdentifier := fmt.Sprintf("%s.success", identifier)
		errorMessages = append(errorMessages, validatePlan(c, subIdentifier, *plan.Success)...)
	}

	if plan.Failure != nil {
		subIdentifier := fmt.Sprintf("%s.failure", identifier)
		errorMessages = append(errorMessages, validatePlan(c, subIdentifier, *plan.Failure)...)
	}

	if plan.Timeout != "" {
		_, err := time.ParseDuration(plan.Timeout)
		if err != nil {
			subIdentifier := fmt.Sprintf("%s.timeout", identifier)
			errorMessages = append(errorMessages, subIdentifier+fmt.Sprintf(" refers to a duration that could not be parsed ('%s')", plan.Timeout))
		}
	}

	return errorMessages
}

func validateInapplicableFields(inapplicableFields []string, plan atc.PlanConfig, identifier string) []string {
	errorMessages := []string{}
	foundInapplicableFields := []string{}

	for _, field := range inapplicableFields {
		switch field {
		case "resource":
			if plan.Resource != "" {
				foundInapplicableFields = append(foundInapplicableFields, field)
			}
		case "passed":
			if len(plan.Passed) != 0 {
				foundInapplicableFields = append(foundInapplicableFields, field)
			}
		case "trigger":
			if plan.Trigger {
				foundInapplicableFields = append(foundInapplicableFields, field)
			}
		case "privileged":
			if plan.Privileged {
				foundInapplicableFields = append(foundInapplicableFields, field)
			}
		case "config":
			if plan.TaskConfig != nil {
				foundInapplicableFields = append(foundInapplicableFields, field)
			}
		case "file":
			if plan.TaskConfigPath != "" {
				foundInapplicableFields = append(foundInapplicableFields, field)
			}
		}
	}

	if len(foundInapplicableFields) > 0 {
		errorMessages = append(
			errorMessages,
			fmt.Sprintf(
				"%s has invalid fields specified (%s)",
				identifier,
				strings.Join(foundInapplicableFields, ", "),
			),
		)
	}

	return errorMessages
}

func validateInputOutputConfig(c atc.Config, job atc.JobConfig, identifier string) []string {
	errorMessages := []string{}

	for i, input := range job.InputConfigs {
		var inputIdentifier string
		if input.Name() == "" {
			inputIdentifier = fmt.Sprintf("%s.inputs[%d]", identifier, i)
		} else {
			inputIdentifier = fmt.Sprintf("%s.inputs.%s", identifier, input.Name())
		}

		if input.Resource == "" {
			errorMessages = append(errorMessages, inputIdentifier+" has no resource")
		} else {
			_, found := c.Resources.Lookup(input.Resource)
			if !found {
				errorMessages = append(
					errorMessages,
					fmt.Sprintf(
						"%s has an unknown resource ('%s')",
						inputIdentifier,
						input.Resource,
					),
				)
			}
		}

		for _, job := range input.Passed {
			_, found := c.Jobs.Lookup(job)
			if !found {
				errorMessages = append(
					errorMessages,
					fmt.Sprintf(
						"%s.passed references an unknown job ('%s')",
						inputIdentifier,
						job,
					),
				)
			}
		}
	}

	for i, output := range job.OutputConfigs {
		outputIdentifier := fmt.Sprintf("%s.outputs[%d]", identifier, i)

		if output.Resource == "" {
			errorMessages = append(errorMessages,
				outputIdentifier+" has no resource")
		} else {
			_, found := c.Resources.Lookup(output.Resource)
			if !found {
				errorMessages = append(errorMessages,
					fmt.Sprintf(
						"%s has an unknown resource ('%s')",
						outputIdentifier,
						output.Resource,
					),
				)
			}
		}
	}

	return errorMessages
}

func compositeErr(errorMessages []string) error {
	if len(errorMessages) == 0 {
		return nil
	}

	return errors.New(strings.Join(errorMessages, "\n"))
}

func validatePlugins(c atc.Config) error {
	errorMessages := []string{}
	names := map[string]int{}

	for i, plugin := range c.Plugins {
		var identifier string
		if plugin.Name == "" {
			identifier = fmt.Sprintf("plugins[%d]", i)
		} else {
			identifier = fmt.Sprintf("plugins.%s", plugin.Name)
		}

		if other, exists := names[plugin.Name]; exists {
			errorMessages = append(errorMessages,
				fmt.Sprintf(
					"plugins[%d] and plugins[%d] have the same name ('%s')",
					other, i, plugin.Name))
		} else if plugin.Name != "" {
			names[plugin.Name] = i
		}

		if plugin.Name == "" {
			errorMessages = append(errorMessages, identifier+" has no name")
		}

		if plugin.Image == "" {
			errorMessages = append(errorMessages, identifier+" has no image")
		}
	}
	return compositeErr(errorMessages)
}
