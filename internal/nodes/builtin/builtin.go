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
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/getglobalvariable"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/gettype"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/getvariable"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/greaterthan"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/htmlextract"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/jsonparse"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/jsonquery"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/length"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/randomnumber"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/regexmatch"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/regexreplace"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/regexsplit"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/reroute"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/typeassert"
	sqlnode "github.com/FlameInTheDark/neuropipe/internal/nodes/database/sql"
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
	"github.com/FlameInTheDark/neuropipe/internal/nodes/flow/setglobalvariable"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/flow/setvariable"
	switchnode "github.com/FlameInTheDark/neuropipe/internal/nodes/flow/switch"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/flow/while"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/displayinput"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/displaymessage"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/displayquestion"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/download"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/form"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/listdirectory"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/readfile"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/writefile"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/math/add"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/math/divide"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/math/multiply"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/math/subtract"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/text/case"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/text/contains"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/text/endswith"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/text/indexof"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/text/join"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/text/replace"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/text/split"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/text/startswith"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/text/substring"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/text/trim"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/twitch/event"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/twitch/sendchatmessage"
)

// RegisterAll registers first-party modules in a stable, reviewable order.
func RegisterAll(registrar nodes.Registrar) error {
	for _, register := range []func(nodes.Registrar) error{
		javascript.Register,
		sqlnode.Register,
		constant.Register,
		formattext.Register,
		getfield.Register,
		buildobject.Register,
		breakobject.Register,
		cast.Register,
		typeassert.Register,
		jsonquery.Register,
		htmlextract.Register,
		equals.Register,
		greaterthan.Register,
		jsonparse.Register,
		getvariable.Register,
		getglobalvariable.Register,
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
		randomnumber.Register,
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
		setglobalvariable.Register,
		returnnode.Register,
		add.Register,
		subtract.Register,
		multiply.Register,
		divide.Register,
		listdirectory.Register,
		readfile.Register,
		writefile.Register,
		download.Register,
		displaymessage.Register,
		displayquestion.Register,
		displayinput.Register,
		form.Register,
		event.Register,
		sendchatmessage.Register,
		split.Register,
		join.Register,
		contains.Register,
		replace.Register,
		trim.Register,
		change.Register,
		startswith.Register,
		endswith.Register,
		indexof.Register,
		substring.Register,
	} {
		if err := register(registrar); err != nil {
			return err
		}
	}
	return nil
}
