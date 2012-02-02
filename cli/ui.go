package main

import (
	"fmt"
	"github.com/bobappleyard/readline"
	"goop"
	//"github.com/nsf/termbox-go"
	"strconv"
	"strings"
	"time"
)

var (
	CLOCK    *goop.Clock
	MIXER    *goop.Mixer
	NETWORK  *goop.Network
	AUTOCRON int64
)

// In general, UI commands should only generate Events, which
// should be sent to items in the X, or sub-interfaces thereof.
// If you're reflecting something you get out of X to anything
// other than an EventReceiver, you're doing something wrong!

func init() {
	CLOCK = goop.NewClock()
	MIXER = goop.NewMixer()
	NETWORK = goop.NewNetwork(CLOCK)
	AUTOCRON = 1
	NETWORK.Add("clock", CLOCK)
	NETWORK.Add("mixer", MIXER)
}

func uiParse(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	if s[0] == '#' {
		return true
	}
	for _, cmd := range strings.Split(s, ";") {
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			continue
		}
		toks := strings.Split(cmd, " ")
		cmd, args := toks[0], toks[1:]
		switch cmd {
		case "quit":
			return false
		case "add":
			doAdd(args)
		case "del", "delete":
			doDel(args)
		case "every":
			doEvery(args) // just an alias for "add cron <autogen ID> ..."
		case "connect":
			doConnect(args)
		case "disconnect":
			doDisconnect(args)
		case "fire":
			doFire(args)
		case "stopall":
			doStopall(args)
		case "sleep":
			doSleep(args)
		case "info", "i":
			doInfo(args)
		default:
			fmt.Printf("%s: ?\n", cmd)
		}
	}
	return true
}

func uiParseHistory(s string) bool {
	rc := uiParse(s)
	readline.AddHistory(s)
	return rc
}

func uiLoop() {
	for {
		line := readline.String("> ")
		rc := uiParseHistory(line)
		if !rc {
			break
		}
	}
}

func doAdd(args []string) {
	if len(args) < 2 {
		fmt.Printf("add <what> <name>\n")
		return
	}
	switch args[0] {
	case "sin", "sine", "sinegenerator":
		add(args[1], goop.NewSineGenerator())
	case "square", "sq":
		add(args[1], goop.NewSquareGenerator())
	case "saw":
		add(args[1], goop.NewSawGenerator())
	case "wav":
		if len(args) < 3 {
			fmt.Printf("add wav <name> <filename>\n")
			return
		}
		if g := goop.NewWavGenerator(args[2]); g != nil {
			add(args[1], g)
		} else {
			fmt.Printf("add: wav: failed: probably bad file\n")
		}
	case "lfo", "gainlfo":
		add(args[1], goop.NewGainLFO())
	case "delay":
		add(args[1], goop.NewDelay())
	case "cron":
		if len(args) < 4 {
			fmt.Printf("add cron <name> <delay> <cmd...>\n")
			return
		}
		delay64, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			fmt.Printf("%s: invalid delay: %s\n", args[2], err)
			return
		}
		c := goop.NewCron(delay64, uiParse, strings.Join(args[3:], " "))
		CLOCK.Events() <- goop.Event{"register", 0.0, c}
		add(args[1], c)
	default:
		fmt.Printf("add: what?\n")
	}
}

func add(name string, item interface{}) bool {
	if err := NETWORK.Add(name, item); err != nil {
		fmt.Printf("add: %s: %s\n", name, err)
		return false
	}
	fmt.Printf("add: %s: OK\n", name)
	return true
}

func doDel(args []string) {
	if len(args) < 1 {
		fmt.Printf("del <name>\n")
		return
	}
	name := args[0]
	item, itemErr := NETWORK.Get(name)
	if itemErr != nil {
		fmt.Printf("del: %s: %s\n", name, itemErr)
		return
	}
	if r, ok := item.(goop.EventReceiver); ok {
		CLOCK.Events() <- goop.Event{"unregister", 0.0, r} // safe
	}
	if err := NETWORK.Del(name); err != nil {
		fmt.Printf("del: %s: %s\n", name, err)
		return
	}
	fmt.Printf("del: %s: OK\n", name)
}

func doEvery(args []string) {
	if len(args) < 3 {
		fmt.Printf("every <delay> <cmd...>\n")
		return
	}
	cronName := fmt.Sprintf("c%d", AUTOCRON)
	AUTOCRON++
	args = append([]string{"cron", cronName}, args...)
	doAdd(args)
}

func doConnect(args []string) {
	if len(args) < 2 {
		fmt.Printf("connect <from> <to>\n")
		return
	}
	NETWORK.Connect(args[0], args[1])
}

func doDisconnect(args []string) {
	if len(args) < 1 {
		fmt.Printf("disconnect <from>\n")
		return
	}
	NETWORK.Disconnect(args[0])
}

func doFire(args []string) {
	if len(args) < 3 {
		fmt.Printf("fire <name> <val> <where>\n")
		return
	}
	name := args[0]
	val64, err := strconv.ParseFloat(args[1], 32)
	if err != nil {
		fmt.Printf("fire: %s: invalid value\n", args[1])
		return
	}
	receiverName := args[2]
	ev := goop.Event{name, float32(val64), nil}
	NETWORK.Fire(receiverName, ev, goop.Immediately)
}

func doStopall(args []string) {
	MIXER.DropAll()
}

func doSleep(args []string) {
	if len(args) < 1 {
		fmt.Printf("sleep <sec>\n")
		return
	}
	delay64, err := strconv.ParseFloat(args[0], 32)
	if err != nil {
		fmt.Printf("sleep: %s: invalid delay\n", args[0])
		return
	}
	<-time.After(time.Duration(int64(delay64 * 1e9)))
}

func doInfo(args []string) {
	for _, name := range NETWORK.Names() {
		what, details := "unknown", ""
		o, err := NETWORK.Get(name)
		if err != nil {
			continue
		}
		switch x := o.(type) {
		case *goop.Mixer:
			what, details = "the mixer", fmt.Sprintf("%s", x)
		case *goop.Clock:
			what, details = "the clock", fmt.Sprintf("%s", x)
		case *goop.SineGenerator:
			what, details = "sine generator", fmt.Sprintf("%s", x)
		case *goop.SquareGenerator:
			what, details = "square generator", fmt.Sprintf("%s", x)
		case *goop.SawGenerator:
			what, details = "sawtooth generator", fmt.Sprintf("%s", x)
		case *goop.GainLFO:
			what, details = "gain LFO", fmt.Sprintf("%s", x)
		case *goop.Delay:
			what, details = "delay", fmt.Sprintf("%s", x)
		case *goop.Cron:
			what, details = "cron", fmt.Sprintf("%s", x)
		}
		msg := fmt.Sprintf(" %s - %s", name, what)
		if details != "" {
			msg = fmt.Sprintf("%s (%s)", msg, details)
		}
		fmt.Printf("%s\n", msg)
	}
}
