// Package builtin provides the deterministic bootstrap for first-party node
// modules. New modules are added here instead of extending catalog switches.
package builtin

import (
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/code/javascript"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/arrayappend"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/arrayget"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/base64decode"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/base64encode"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/breakobject"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/buildobject"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/cast"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/chathistory"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/constant"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/equals"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/formattext"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/getfield"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/gettype"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/getvariable"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/greaterthan"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/jsonparse"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/jsonquery"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/length"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/regexmatch"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/regexreplace"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/regexsplit"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/reroute"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/typeassert"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/date"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/flow/branch"
	breaknode "github.com/FlameInTheDark/neuropipe/internal/nodes/flow/break"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/flow/doonce"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/flow/flipflop"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/flow/foreach"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/flow/forloop"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/flow/gate"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/flow/multigate"
	flowreroute "github.com/FlameInTheDark/neuropipe/internal/nodes/flow/reroute"
	returnnode "github.com/FlameInTheDark/neuropipe/internal/nodes/flow/return"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/flow/sequence"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/flow/setvariable"
	switchnode "github.com/FlameInTheDark/neuropipe/internal/nodes/flow/switch"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/flow/while"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/listdirectory"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/readfile"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/writefile"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/math/add"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/math/divide"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/math/multiply"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/math/subtract"
)

// RegisterAll registers first-party modules in a stable, reviewable order.
func RegisterAll(registrar nodes.Registrar) error {
	for _, register := range []func(nodes.Registrar) error{
		javascript.Register,
		constant.Register,
		formattext.Register,
		getfield.Register,
		buildobject.Register,
		breakobject.Register,
		cast.Register,
		typeassert.Register,
		jsonquery.Register,
		equals.Register,
		greaterthan.Register,
		jsonparse.Register,
		getvariable.Register,
		chathistory.Register,
		reroute.Register,
		arrayappend.Register,
		arrayget.Register,
		base64encode.Register,
		base64decode.Register,
		gettype.Register,
		length.Register,
		regexmatch.Register,
		regexreplace.Register,
		regexsplit.Register,
		date.Register,
		branch.Register,
		sequence.Register,
		foreach.Register,
		forloop.Register,
		while.Register,
		switchnode.Register,
		doonce.Register,
		gate.Register,
		flipflop.Register,
		multigate.Register,
		flowreroute.Register,
		breaknode.Register,
		setvariable.Register,
		returnnode.Register,
		add.Register,
		subtract.Register,
		multiply.Register,
		divide.Register,
		listdirectory.Register,
		readfile.Register,
		writefile.Register,
	} {
		if err := register(registrar); err != nil {
			return err
		}
	}
	return nil
}
