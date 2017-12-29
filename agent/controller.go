package agent

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// RPC exposes methods for RPC call.
type RPC struct {
	controller Controller
}

// Controller stands for a single agent and exposes management interfaces.
type Controller struct {
}

// Invocation represents a command invocation from the master to the agent controller.
type Invocation struct {
	Command   string
	Arguments []string
}

var availableCommands = make(map[string]reflect.Method)

func (c *Controller) DoSetup() error {
	return nil
}

func argError(pos int, command string, expected string, given string) error {
	return fmt.Errorf("The %dth argument for command '%s' is %s, but it cannot be parsed from '%s'", pos, command, expected, given)
}

// Invoke calls the method on the agent controller with the name Do<Command>.
func (rpc *RPC) Invoke(invocation *Invocation, reply *interface{}) error {
	if invocation == nil {
		return fmt.Errorf("nil Invocation")
	}

	method, ok := availableCommands[invocation.Command]
	if !ok {
		return fmt.Errorf("Command '%s' was not found", invocation.Command)
	}

	argsCount := method.Type.NumIn()
	if len(invocation.Arguments) != argsCount-1 {
		return fmt.Errorf("Command '%s' needs %d arguments, %d provided", invocation.Command, argsCount, len(invocation.Arguments))
	}

	in := make([]reflect.Value, argsCount)
	in[0] = reflect.ValueOf(&rpc.controller)
	for i := 1; i < argsCount; i++ {
		stringArg := invocation.Arguments[i-1]
		t := method.Type.In(i)
		var arg interface{}
		var err error
		switch t.Name() {
		case "string":
			arg = stringArg
		case "int":
			arg, err = strconv.Atoi(stringArg)
			if err != nil {
				return argError(i, invocation.Command, t.Name(), stringArg)
			}
		case "float32":
			tmp, err := strconv.ParseFloat(stringArg, 32)
			if err != nil {
				return argError(i, invocation.Command, t.Name(), stringArg)
			}
			arg = float32(tmp)
		case "float64":
			arg, err = strconv.ParseFloat(stringArg, 64)
			if err != nil {
				return argError(i, invocation.Command, t.Name(), stringArg)
			}
		// TODO: Support more types
		default:
			return fmt.Errorf("The %dth argument type %s for command '%s' is not supported", i, t.Name(), invocation.Command)
		}
		in[i] = reflect.ValueOf(arg)
	}

	response := method.Func.Call(in)
	*reply = response[0].Interface()

	return nil
}

func init() {
	tpe := reflect.TypeOf((*Controller)(nil))
	for i, total := 0, tpe.NumMethod(); i < total; i++ {
		method := tpe.Method(i)
		if strings.HasPrefix(method.Name, "Do") && method.Type.NumOut() == 1 {
			availableCommands[method.Name[2:]] = method
		}
	}
}
