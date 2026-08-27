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
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/base64ext"
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
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/pathops"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/randomnumber"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/regexmatch"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/regexreplace"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/regexsplit"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/reroute"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/data/typeassert"
	uuidnode "github.com/FlameInTheDark/neuropipe/internal/nodes/data/uuid"
	kvcommand "github.com/FlameInTheDark/neuropipe/internal/nodes/database/kv/command"
	kvhash "github.com/FlameInTheDark/neuropipe/internal/nodes/database/kv/hash"
	kvkeys "github.com/FlameInTheDark/neuropipe/internal/nodes/database/kv/keys"
	kvlist "github.com/FlameInTheDark/neuropipe/internal/nodes/database/kv/list"
	kvscan "github.com/FlameInTheDark/neuropipe/internal/nodes/database/kv/scan"
	kvset "github.com/FlameInTheDark/neuropipe/internal/nodes/database/kv/set"
	kvsubscribe "github.com/FlameInTheDark/neuropipe/internal/nodes/database/kv/subscribe"
	kvzset "github.com/FlameInTheDark/neuropipe/internal/nodes/database/kv/zset"
	sqlnode "github.com/FlameInTheDark/neuropipe/internal/nodes/database/sql"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/date"
	discordaddreaction "github.com/FlameInTheDark/neuropipe/internal/nodes/discord/addreaction"
	discorddeletemessage "github.com/FlameInTheDark/neuropipe/internal/nodes/discord/deletemessage"
	discordeditmessage "github.com/FlameInTheDark/neuropipe/internal/nodes/discord/editmessage"
	discordevent "github.com/FlameInTheDark/neuropipe/internal/nodes/discord/event"
	discordsenddm "github.com/FlameInTheDark/neuropipe/internal/nodes/discord/senddm"
	discordsendmessage "github.com/FlameInTheDark/neuropipe/internal/nodes/discord/sendmessage"
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
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/archive/unzipfiles"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/archive/zipfiles"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/displayinput"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/displaymessage"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/displayquestion"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/download"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/base64tofile"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/copyfile"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/copyfolder"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/deletefile"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/deletefolder"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/fileexists"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/filetobase64"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/folderexists"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/movefile"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/movefolder"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/renamefile"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/renamefolder"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/waitforfile"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/fileops/waitforfolder"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/form"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/listdirectory"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/readfile"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/local/writefile"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/math/add"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/math/divide"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/math/multiply"
	"github.com/FlameInTheDark/neuropipe/internal/nodes/math/subtract"
	telegramanswercallback "github.com/FlameInTheDark/neuropipe/internal/nodes/telegram/answercallback"
	telegramchataction "github.com/FlameInTheDark/neuropipe/internal/nodes/telegram/chataction"
	telegramdeletemessage "github.com/FlameInTheDark/neuropipe/internal/nodes/telegram/deletemessage"
	telegrameditmessage "github.com/FlameInTheDark/neuropipe/internal/nodes/telegram/editmessage"
	telegramevent "github.com/FlameInTheDark/neuropipe/internal/nodes/telegram/event"
	telegrampinmessage "github.com/FlameInTheDark/neuropipe/internal/nodes/telegram/pinmessage"
	telegramsendmessage "github.com/FlameInTheDark/neuropipe/internal/nodes/telegram/sendmessage"
	telegramsendphoto "github.com/FlameInTheDark/neuropipe/internal/nodes/telegram/sendphoto"
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
		kvkeys.RegisterGet,
		kvkeys.RegisterSet,
		kvkeys.RegisterDelete,
		kvkeys.RegisterExists,
		kvkeys.RegisterIncrement,
		kvkeys.RegisterRename,
		kvkeys.RegisterExpire,
		kvkeys.RegisterTTL,
		kvhash.RegisterGet,
		kvhash.RegisterSet,
		kvlist.RegisterPush,
		kvlist.RegisterPop,
		kvlist.RegisterRange,
		kvset.RegisterAdd,
		kvset.RegisterMembers,
		kvset.RegisterRemove,
		kvzset.RegisterAdd,
		kvzset.RegisterRange,
		kvzset.RegisterRemove,
		kvscan.RegisterScan,
		kvscan.RegisterPublish,
		kvscan.RegisterInfo,
		kvcommand.Register,
		kvsubscribe.Register,
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
		discordaddreaction.Register,
		discorddeletemessage.Register,
		discordevent.Register,
		discordeditmessage.Register,
		discordsenddm.Register,
		discordsendmessage.Register,
		telegramanswercallback.Register,
		telegramchataction.Register,
		telegramdeletemessage.Register,
		telegramevent.Register,
		telegrameditmessage.Register,
		telegrampinmessage.Register,
		telegramsendmessage.Register,
		telegramsendphoto.Register,
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
		// Archive
		zipfiles.Register,
		unzipfiles.Register,
		// File ops
		fileexists.Register,
		folderexists.Register,
		waitforfile.Register,
		waitforfolder.Register,
		copyfile.Register,
		copyfolder.Register,
		movefile.Register,
		movefolder.Register,
		deletefile.Register,
		deletefolder.Register,
		renamefile.Register,
		renamefolder.Register,
		filetobase64.Register,
		base64tofile.Register,
		// UUID
		uuidnode.RegisterGenerate,
		uuidnode.RegisterParse,
		uuidnode.RegisterValidate,
		uuidnode.RegisterExtract,
		// Path utilities
		pathops.RegisterGetPathPart,
		pathops.RegisterBuildPath,
		pathops.RegisterCleanPath,
		// Extra base64 (pure bytes↔text)
		base64ext.RegisterBytesToBase64,
		base64ext.RegisterBase64ToBytes,
	} {
		if err := register(registrar); err != nil {
			return err
		}
	}
	return nil
}
