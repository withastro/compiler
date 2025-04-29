package printer

import (
	"strings"
	"testing"

	astro "github.com/withastro/compiler/internal"
	handler "github.com/withastro/compiler/internal/handler"
	"github.com/withastro/compiler/internal/transform"
)

// One of the more performance-sensitive parts of the compiler is the handling of multibytes characters, this benchmark is an extreme case of that.
func BenchmarkPrintToTSX(b *testing.B) {
	source := `🌘🔅🔘🍮🔭🔁💝🐄👋 🍘👽💽🌉🌒🔝 📇🍨💿🎷🎯💅📱🎭👞🎫💝🍢🕡 augue tincidunt 👠💋🌵💌🍌🏄🕂🍹📣🍟 🐞🍲👷🗻🏢🐫👣🐹🔷🎢👭🍗👃 🐴🐈📐💄 et, 👽👽🔙🐒🔙🍀 🐞🍐🎵💕🍂🍭🎬🎅 ac 🏃👳📑🐶👝🔷 🔕👄🐾👡🍢💗 🌓🔙🌔🏈🔒🔄🎹🎐 🎾🐝🌁📃💞📔🔕🐕 vel 🌟🏁💴🎾🔷📪💼👣📚 👟💻🗾🎋💁🏬🐮💑 🍈🌸🍓🍥💦 et vivamus 🍑🍫🔉🔹💽🍙 rhoncus eu 📰💫💀🌺 🔋💾🔱🔷🐟 convallis facilisi vitae mollis 📨🔚💮🔃🎀🍶 🌠📑🎹🌑📫🗽🐊💁🔼🕓 ac 🌂🌳🌺🎭🍧🐑 🔭🔉💉🍗👠🍦 🔄👔🎭🍇 🏤🍂🌝👜🔺 ornare 🔽🌰🎃💝👩 🐬🌻🍩👺🎆📣 risus 🌼👧🌒🍄💄🌆🐖👐📠 🕙🍈👞🏢👅🎽🏫👃🌾🍘🕑🎼💆🎳 🐻📵🔂🍩🕦👕🐶 💚🏮📟📈🌄👱🔚 sem 🔵💱💭💫 libero bibendum 🌿🍮🎧🍴💉 🐵🔷🍒🍜 sed metus, aliquam 🐷🐬📇👔🎴🔻🏊📴🍂🎽 🏀💡🍺🌾 💣🍇🐼🌀🐟🍂🐰 sed luctus 👾🍠👻💬📋🐈👀🌕💥🐹 consectetur commodo at 📓🐣🔮🐍🔺🍐🗻🍃🍠🔬 🏁🎬🐔🌙 🎿🎊🍳💓👹🐏 👦🐩👶💻 🔉💏📧🔲🍈📹💫🎧 🌟🍯🔭🎿 🎹🗻💳🔄🕁💂 🐩👠🌗🍹 facilisis 👍🍑💤🐕🐞🐻🏩 posuere 🎑🍢🍑🏨📗👣 ultrices. Vestibulum 🌠👰💼📐🍺💫 👘🐖🔕🔤🐖💶🐢📵👍🍪 🔸🎿🔤🍡💡🌄📁👉🎎🎆🎢🔒 📴👉🌱🐍🌓🎆🏩🐀👓💹🎴 🔴🌒🔘👡🕜 🌝🐉🐑🏮🎸🐳💉🎄 🍳🐲🔭📆🐎🔼🎐🐩💾🍈🎶💅🐜🍀. 🍭🍄🌳🍏💍👈🌆 🔦🏦👵🌹🏊🎐📳🌃📘🌷💎📓🎼 🎳🔁🔂🗽 💅🏰🐊👂🌴 💍🕗🐘🔼🔰🏀 🎂📻🕛🍔🎄 vitae 📎🐆🍚🌲🐰 ipsum 👘👎📅🎈🌆💮🔁🎤 💺💉💉👐🔛🐛🐺🍸📭 💠👞🍨💖 integer 🐌📀🍂🍍🐼 volutpat, condimentum elit 🎌🌕🏊👼 🌛🕜🌚🕤🍎 🏢💨👓👤 duis amet 📹🕚🔏🏧🌷🍄👦 🔠🐬👬🍩📫🏧🎷🔢💆🎭🔳👈 🔇👿💈👧🐍🍱🔉 in 🌐🎲🌠🌟🐀👛🌞🎧 sed. Tristique malesuada id 🏆🍕🕚🎯🌶 📺🐘🏀🐮🔶🏀🍥👬💞🎃🌙🎥🔦🍗 👘👣🐂🎪🔂🎾 👎👮💈🗻🌿💰📩🔋💭💃 🍍🕑🔋👠🍆🍈👰🌅 orci nam 🔀🕠🏄🍣👘🐲💘🐥 🏥🎊🎯👾📅 👛🌽🏫🔃👋🐀👶💥🔳 🕥👪👑👯 🐓💡🔠💼🔳💲 👬🔁🍜💘🔌🌎🏃🌶🍏🍯🎺 🔟📨🔜🎱💓🔛 accumsan 🎢🐭🍉🍳🕜🏆🔗📝👿 🍗👑📜🎁🍇🍕🌛 🎓🌰👣📆🌋 👌🍱💏🔇🐆🌞📝💻🌺 dictumst 📡🍝🌐💤🐅🔔🐟📥🌓🌒🍅📨🏡👺🍬🐟 🔹💳🍨💡🎋💕🕜🏦👟🕒🕣💅💗 🏤🔈📀📜📙🍌 📫🕘👍💾📱 purus 🕒🐕👵🏄💗🐤🕝 pharetra adipiscing elit, non 🏬👸🍉👑 🎠🌇👰💄 🌕🗼🏮💇🐗 📌🏤🌲👹📌 🎐🕓📎🏤💾 tellus 💆🍨👂🕟💚🎋🌠🐳 🍘🍫🎵📚📟🔛🐑🏭🔄👌📏👚🏄🐞 💏🍊👾🍘👰📄💯🔑 proin 🐡🔝🐳🎻🐶🍜 🍕🌵📯🔖💅🕔💘🏁🌾💨🔲🔼🍜🍘🕂 in suscipit 🍎🎄🎬🔙👪💣🍣💯🕧 lectus 🌾🍚🍺🐆💉🎡👷 📖🔅👼🕞 🌄🌃📦🔲💘🌶💁🍯💿 senectus 👻💈🗽🍈 📜🍉🍒🍐🔖👙🔀🐅👙 🐬📷🎨👹🎬 🐷🌐🔽💨🎿🌌🌒 🎎🍊🔕🍁. 🍈🌄📧🏊📛👗🕟 🍋💌🔁🐛💫🔰👃🕑 ridiculus mattis 🕓🍌👘🌹 👅💏🍨🔯🍂🕃🌝👠🏁🔔 pellentesque elit eu, 💝🍧💯🔄📈📛🐻📆🔱📴🔸🍻🐘🎀 viverra 🌆🍉🍉🌆 📪🍕🏊🌜📺🔆🐢🎃 mi sed tellus luctus 🍜🍃🔶🗽 laoreet dui tristique 🔆🌸🐛🔣🏤📘🕜🍃🐋 pretium ultrices 🔳🏇🍏🎭🍀 🍱🐠👬📮🌗🌆💺🔆👨🌱 et 👸📁📩🔌👯👏👫👳💸 🐎💉🔱💦👴 🏪🍶🐸🔤 💔🏤👺👻🏤🎥🐽🔦👌🔡📚💡🔁🎻 🎵👡🕀🎩 🍸🐒🍭🕠🔢🎧🍚💪 🌸🍰💏🏁🎥🐕🔬💶 💃🌽💕🐭 💯🍁📭🐚📛📓🌏🔯📦 🕝👾📒📳💵 🏠💺🕔🕁📃 adipiscing nulla congue 🔖💲🕚🎰🔋👬 sem 👽👵👜👇 vehicula 📑🌎🍁💈 consectetur nulla ullamcorper enim, 🏫🍃🐢🔄🍝🕢👖💨👺💰👎💳🏠🏤 vel fermentum porttitor lacus 📱💧🔜🔰👱🎰🐉🔓🕧🍌🕙 🎴🍫🐱🐟🌖🕞🏥🍝📜🔃🏈🔡🐁🔗 💊🍐📝👭🎢🐳🐯📷🐀🔃. 🐦🍚🔵🕜🏩 id 🌔🔤💿🌻🍹 🌎🎈📢🔬🐖💢👸🎭 🍤🔵🔓🕕📪💢🔁👽💴 🐛👶🔢🎹🏄👜📌🍭🔼🎫🍯🎦💎🐦 🐨💧🍴🌊🔁 consequat pretium 👍🏬🏰👖🍥📺👛🌕 🏄🔱🔩🐏🌞🌺🐌👑 🏰🕢💱👖🐶🐥📯🎶💧🍭🔀🎾🏩🍑 massa, est 👶🍭💅🕔🍳💗🍞💚🍩🐷🍘🌄🐇 🐼👀👀🎅 📄🎁🐞💼 placerat erat 💧🌉💻🔣🐥📠🍪📻🐃🔻👠🕘🍞🍈🍔📴 dolor 📁💹🎐🍂🍉 neque, 👗🍁🍪🌵🕛🍄 🕝👔🔼🎡🔺📬 sed 💨💧🏢🐼👘🌳🎼👹🐉🕕 enim 🔏🏁👊💀🔓 🏫🐈💀🌳🔒👵🔘🎺💴🍑 👭🔲🍰💿🔪🌊 🌅🕔🎑📛🔶🔘🎑🍯 🐲📺👬📊🍒 elit, 🌽🎱💇🎥 ultricies 📡💲🐦🌁🍚🌵🍵 🎮📧🔟🍴👕🔏🌴🔊👳🗼💒🌴👍📞🔳👜🔥🐝🐾🐧🍊 🌰🗾🌂🐄 🔛🐀🍏🍞🔔📖💉📼🐥🍱 🐜🎺🍵💿👂🌋🌸 🔞🐤🔟🍀📙🏩👑 porta id 🎄💺🍙👶👪🔪👪🐭💚📗🎅🍐👡🎭 🐦🎌🐆💫 quis nulla dictumst non 📫🍵🔫🕠 🔯🔃🐷📖💙👓💍 🎬🕃🐥🏫🍛🌆🐗👅📷 📚🍭🌞🌎🕐👜👳 🏧💬💴🎆🐄🔠📄🐰📝🌈 🎩🌇🌟📙 suspendisse 🔡👫🍐🏩🌉🕚📘🐮 👐🏁👮🎭🏣 👰📤🍙🏈👓🐰🍦 lacinia diam eu vestibulum donec faucibus 🔝🎴🌷💩🍡🍜💙 🎐🔉🌛📠🏢📄🍆🎆.

<script>console.log("That's a lot of emojis.");</script>

🔲🍀🏨👆👎🏩🏦🎹 nibh 🕞🍃🔻📨 nec pulvinar 📏🐜💻🐸🐾🐾💖 risus 🍫🕜🐑🌳📇🍋🐪🎣 neque 👝🐝🌹🍫🌌👅 👹🏈🗽🐣 💉📊🐘🔉🏆💡🌸👰🔛📼 🍺🌿🐪🔜💄🎒👟 amet vitae, morbi elit rhoncus 🐙🏫🍪🎡👕💵 🌕💁💯🐷🌾📼🐀🍬🌛🕤🐊👔🔩🍂📚💵 💖🐐👹🔼🏠🍢🎼🐧 👛📩🍯💊 📞🌒📞💽 purus 🔐📼📬🌞🍁💸 morbi 🍁🎾🎡🌋 📨🌈🔈🌆🔨📕💛🏡🏯 💅🎣👽🕦 🌴🍋🔐🔑 ut congue 🔊🐧🌻🕑 gravida 🐉👅👦👢🎉🔎💪🔄🎈 👳🏡🕥🎂 🔵🐘🔠🍑👉🔀📊🏡 🌄🍯💽📁📚🐆👆🌂🎡📖🎱👮🌽🐄 orci 📚🍣🐀🎦📪💶🌓 etiam 🌑🐁💇👬🍓📅💸👟🌕🐊🎁🏪 vehicula sed 📒📭📣🌌🐁🎲🌴🏠🔏👯📥💽🌗💲🍡🔫 🎈🏥🍇🔺💍💐🌳👎🎤🍮📭📊 🍓👠🕞🕘🍂🏣🐺🍬🐖 eget lectus 🍄🍍📉📥🌾🎴🏣 🐾🕤🍼🌃🔩🐂🕣🐉💧📊🎧🎧🌂🎠 🐵🌺📰📑🏰 👣🍸💚🐗🔜🕕🐠🕙🏇📲🌙 👫💛💽🐑💸 🔐🔝🔘🎪💻🔂 nunc, 🍹🕗🏡📤🎷 sem vitae adipiscing tempor, 🕡🍚🐈🌟👎💢🔦 🌴💿💔🎳📍🌽🎒🔭🔨 lectus 🌰🌓🗽💀🍈 est 📬💑🕁🍺 📀🏮🔫🔜🎭🌷🏀💑🍂🔵 vulputate leo eget 🍗🌴🎃🔣👲📙 📺🍈🍏💦🌅💔💌 phasellus 👨💡🔯🐃🍜🐒🔵💆🍩🐦🍬🍅🍧 🎲🍰💊🏣📁💎 🔮🎉🎱👿🐟💪🕤 🍡🐜🕚🌏. Turpis vulputate 🌗🔫📙🎽🎽🎿📓 pretium congue in arcu tincidunt. Nisi 📺💍👃🕃🐫 🐝📨🍁🕕💯💭 📬🏧🔧🌟 🏩👛🏭🌽🎮🌁 magnis porttitor 🔈🎄🌓👶🏮. 👢🌵🏬🌏🍩 rutrum egestas 💙🔠🎧🐜🎣 nisi lectus feugiat 🍀🕚🕢🌀🎰💅 💝📮🌝📃🔈 🎓📝🏮🍄📢📛🍺🔊💐 sagittis 🐖🌂🎓👒👎🔼👊📣📭👿🐦🔖🍵💺🐳 🕗🔭🐭📐👍💯 massa erat 🏧🐁🎤👔💑🍣🐢 🍐🗾🔙🍊🎭🔣👐 💊💖🌳💌💿🏯🔴🎪. Id 🐅🐦💝📱💐👓🎡 ut 🏬🔔🌟🌑 🎦💮🎩🔬 👰📵🍘🎴 👪🐩👳💆🍧 purus 💮🐦🐼🎷🔦 🎺🔇🍴📈 cras pretium volutpat, etiam risus 🐇🕔🐪🔽 👠🔮🔈🎃🎧🌄👜 vel sapien 💯👻👜🎼📜 🎽💀🏊🌉📴🍻📐💉📺🐺 vivamus lorem 📃📉🔞🕦 📣🍨🌉🔩🐺🔎👙 molestie tellus 👹💃🕕📗🔵🕢 vel 📎🐍🔅📁🔁🍚🌀💏🐦 🎱🐮🕣👋📳🎑 🕙🏡🕒📖📪👩 condimentum 🐤💐💷🕞💬💝🎨💰 amet nisl fringilla bibendum 🐘🏃🎃🐉🏰🏦🐎🎱🍅🎥🔳🎵🍠🏧🍖🔭💇 🕞🎱📂🐈👇 🍯👐🎹👘💯📗👷 💂🕘👦💘💆🌗🕃🏀🔚 🔜💃🌒🍧🔝🐹🔑💂👜🐭 🎤👌👐🏩📞🕦🔜🍙 morbi 👑🐸🌄🐹🐃🐢🍳💸 malesuada quam amet, 💰🔗👗🐰🍆🕑📮 🍋🌘🐞🍧 🔻🎺🍘👐🍬🔷 pulvinar 🔭🏬👦💆 vivamus tempus 🏧👉🕢📭🔠🌔🕧💧📊🔼 👬🐑🌘🍮🎎🐝🕃👗🔴🐽🐘🌈📺👙🕝.

<div onload="console.log('It really is.')"></div>

Faucibus 🎈🐋🔄📇🐡💐 🎾🎩🔹🔣🎍🐸🌳 vestibulum, 🐢🌘🕜👂💬🎑🍪 📫📊🎅🌷📝🔰🏣💭👧👽🎒📕💓👯 🎎🍁🕥🕑💗🐌📩📧🍸🌙 🕝🐵🐀🐫🎫🔰👲🍛🔵🍪 🍳🏆🎷🐐 quam 💠🎈🕓🌴🌱🍨💢🍮🕡🔡💎🐍 imperdiet placerat 🔱🐑📔🔧🌍 nisl 📢🗻🐹🔏🕕🐐💦 📈🔵📴🍳📬💕📧📓🍚 📈🌸🔱💜💎🌐🍻📘🏠💵🏡🔌📦🐑🍩🔇👅📎🐆 🐌💞📺🐫🍷🌍💒🌸🔎🌇📠🔂🌼💎🍟 ut vel et 🎩👖💾👢🎵💂 🌂📱💳🐨🍲 🍔🔵🏃🎦🕐🌟🌐🔑 tristique vel 🌃💋👉🔰. Tortor sit 🌓👑🔓🐀🌹 tempus 🍸🍹📔🎂🍺🏀 consequat ornare 👽👽🍃📗🌘🍍 🔯🌲📝📥🍐🏣🐸 🍔📝📞🍣 💩🔥💨👋🔹 🌕💊🏪🕡 🏇🏉🐵👢🎳🕛🍸 🐈🏇🏭🕁🍬 rhoncus 📛🍁🔨💶 bibendum 🎋👲🏤📰🍐 🌸📙🍏🌠 🎿🎀👡💲📋💦🐵🔑🕡🕞🍥🍍👧🔘💡 arcu 💣👍🌶👬🌹🔒🌁📠🕖🎓📌🎩📫💬 👋🌚🌶💶🍊💫🔁📲🎺🐮💙🍟 🕦🌁🐡🍵🍒 🍁🍜🍪🔳📞💝💻🎶📦🔵📯🏦🐎 lobortis malesuada 💇💸🏰💅. 🌐🐕🐂📇 🎒💨🔙🐉🎹🔥🔗📴👥🎈📒🔸💍🔇🌙🕁🐊🏣💆💗🔽 tortor 🔵🐚🍓🌱🌆🐀🐻 🔓👦🍌🌔🍯👐🕣 🌅👇📝💰💝 condimentum 🍋💉🐞📆🍲👢🐬💌🎤 🎢👷👑👆 💱🎒🏬🎫 🗾💝👎👄 💊🔅🔙🐮🍗🐥 nulla adipiscing 🎦👿🐞🔋📜👵 🏄🐷🎵👾 🎿🍒🌲💄📚🌔📭👄👿🍱📷💮🔀🍄 velit.`
	for i := 0; i < b.N; i++ {
		h := handler.NewHandler(source, "AstroBenchmark")
		var doc *astro.Node
		doc, err := astro.ParseWithOptions(strings.NewReader(source), astro.ParseOptionWithHandler(h), astro.ParseOptionEnableLiteral(true))
		if err != nil {
			h.AppendError(err)
		}

		var fmContent []byte
		if doc.FirstChild.Type == astro.FrontmatterNode && doc.FirstChild.FirstChild != nil {
			fmContent = []byte(doc.FirstChild.FirstChild.Data)
		}
		s := js_scanner.NewScanner(fmContent)
		PrintToTSX(source, doc, s, TSXOptions{
			IncludeScripts: false,
			IncludeStyles:  false,
		}, transform.TransformOptions{
			Filename: "AstroBenchmark",
		}, h)
	}
}
