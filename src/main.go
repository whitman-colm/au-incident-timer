package main

import (
	"fmt"
	"github.com/aurumbot/flags"
	"github.com/aurumbot/lib/dat"
	f "github.com/aurumbot/lib/foundation"
	dsg "github.com/bwmarrin/discordgo"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type topic struct {
	Name             string    `json:"n"`
	Triggers         []string  `json:"t"`
	CurrentStreakSet time.Time `json:"s"`
	LastStreakSetter string    `json:"ls-u"`
	LastStreakSet    time.Time `json:"ls-t"`
	HighScoreSetter  string    `json:"hs-su"`
	HighScoreSet     time.Time `json:"hs-st"`
	HighScoreBreaker string    `json:"hs-bu"`
	HighScoreBroken  time.Time `json:"hs-bt"`
	Message          string    `json:"m"`
}

var config struct {
	Guild map[string]struct {
		Topics map[string]topic `json:"topic"`
	} `json:"guild"`
}

var Commands = make(map[string]*f.Command)

var triggerReply = make(map[string]struct {
	LastTrigger  time.Time
	MessageCount int
})

func init() {
	Commands["bpconfig"] = &f.Command{
		Name: "The Bad Post Configuration Tool",
		Help: `bpconfig is the modificaation tool for the bad post timer. It is used to define
topics and triggers (as regular expressions). If you have retained vestages of
your sanity and don't want to learn regex, then TL;DR to match "word" not within 
other words, use \b(word)\b
If you're a regex elder god, it uses https://github.com/google/re2/wiki/Syntax (minus \C)
# Arguments
**--new <topic name>** : Create a new topic with the topic name <topic name>
**--del <topic name>** : Delete <topic name>. Dangerous!
**-listtopics** : List all topics
**<topic name> <--add|-a> <regex>** : Add a trigger to <topic name> fulfilling <regex> (one regex per --add is supported). Case insensitive.
**<topic name> <--rm|-rm> <item number>** : Remove the <item number> trigger from <topic name>
**<topic name> <-ls>** : List all topic triggers
# Usage:
` + f.Config.Prefix + `bpconfig -new foo
` + f.Config.Prefix + `bpconfig foo -a ^[A-z].+$`,
		Perms:   dsg.PermissionManageWebhooks,
		Version: "v1.0.2",
		Action:  bpconfig,
	}

	dat.Load("incident-counter/config.json", &config)

	if config.Guild == nil {
		config.Guild = make(map[string]struct {
			Topics map[string]topic `json:"topic"`
		})
	}

	f.Session.AddHandler(incidentHandler)
}

func incidentHandler(s *dsg.Session, m *dsg.MessageCreate) {
	if m.Message.Author.Bot {
		return
	}
	if strings.HasPrefix(m.Message.Content, f.Config.Prefix) {
		return
	}
	m.Message.Content = strings.ToLower(m.Message.Content)
	var guildID string
	if channel, err := s.Channel(m.Message.ChannelID); err != nil {
		dat.Log.Println(err)
		return
	} else {
		guildID = channel.GuildID
	}
	if tmp, ok := triggerReply[m.Message.ChannelID]; ok {
		tmp.MessageCount++
		triggerReply[m.Message.ChannelID] = tmp
	} else if !ok {
		tmp.MessageCount = 0
		triggerReply[m.Message.ChannelID] = tmp
	}
	// Local topics reduces verbosity by grabbing the config.Guild[guildID] value.
	guildTopics := config.Guild[guildID]
	if guildTopics.Topics == nil {
		guildTopics.Topics = make(map[string]topic)
	}
	defer dat.Save("incident-counter/config.json", &config)
	for li := range guildTopics.Topics {
		for i := range guildTopics.Topics[li].Triggers {
			match, err := regexp.MatchString(guildTopics.Topics[li].Triggers[i], m.Message.Content)
			if err != nil {
				dat.Log.Println(err)
				return
			}
			if match {
				guildTopics.Topics[li] = matchedTopic(guildTopics.Topics[li], s, m.Message)
				return
			}
		}
	}
	config.Guild[guildID] = guildTopics
}

func bpconfig(session *dsg.Session, message *dsg.Message) {
	var guildID string
	if channel, err := session.Channel(message.ChannelID); err != nil {
		dat.Log.Println(err)
		return
	} else {
		guildID = channel.GuildID
	}
	// Local topics reduces verbosity by grabbing the config.Guild[guildID] value.
	guildTopics := config.Guild[guildID]
	if guildTopics.Topics == nil {
		guildTopics.Topics = make(map[string]topic)
	}
	defer dat.Save("incident-counter/config.json", &config)

	flgs := flags.Parse(message.Content)
	var topicname string
	for i := range flgs {
		if flgs[i].Name == "--unflagged" {
			topicname = flgs[i].Value
		} else if flgs[i].Name == "--new" {
			newTopic := topic{
				Name:             flgs[i].Name,
				Triggers:         []string{},
				CurrentStreakSet: time.Now(),
				LastStreakSetter: "478626533636833282",
				LastStreakSet:    time.Now(),
				HighScoreSetter:  "478626533636833282",
				HighScoreSet:     time.Now(),
				HighScoreBreaker: "478626533636833282",
				HighScoreBroken:  time.Now(),
				Message:          "This is the Default Message!",
			}
			newTopic.Name = flgs[i].Value
			guildTopics.Topics[flgs[i].Value] = newTopic
			session.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Created topic %v.", flgs[i].Value))
		} else if flgs[i].Name == "--del" {
			delete(guildTopics.Topics, flgs[i].Value)
			session.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Deleted topic %v.", flgs[i].Value))
		} else if flgs[i].Name == "--listtopics" {
			var str = "**Topics for this server:**"
			for i := range guildTopics.Topics {
				str += "\n- **" + guildTopics.Topics[i].Name + "**"
			}
			str += "\nUse `" + f.Config.Prefix + "bpconfig <topic> -ls` to list assosiated triggers or `" + f.Config.Prefix + "help bpconfig` for more info."
			session.ChannelMessageSend(message.ChannelID, str)
		}
		if topicname != "" {
			switch flgs[i].Name {
			case "--add", "-a":
				_, err := regexp.MatchString(flgs[i].Value, "Some String")
				if err != nil {
					session.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Hey nerd, **%v** doesn't make any sense. Try again idiot. `(error: %v)`", flgs[i].Value, err))
					continue
				}
				tmpTopic := guildTopics.Topics[topicname]
				tmpTopic.Triggers = append(tmpTopic.Triggers, flgs[i].Value)
				guildTopics.Topics[topicname] = tmpTopic
				session.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Created uh..... trigger `%v`.", flgs[i].Value))
			case "--remove", "-rm":
				index, err := strconv.Atoi(flgs[i].Value)
				if err != nil {
					dat.Log.Println(err)
					dat.AlertDiscord(session, message, err)
				}
				tmpTopic := guildTopics.Topics[topicname]
				tmpTopic.Triggers[index] = tmpTopic.Triggers[len(tmpTopic.Triggers)-1]
				tmpTopic.Triggers = tmpTopic.Triggers[:len(tmpTopic.Triggers)-1]
				guildTopics.Topics[topicname] = tmpTopic
				session.ChannelMessageSend(message.ChannelID, fmt.Sprintf("Trigger #%v committed not in memory.", flgs[i].Value))
			case "--list", "-ls", "-l":
				str := fmt.Sprintf("List of triggers for %v:", topicname)
				for i := range guildTopics.Topics[topicname].Triggers {
					str += fmt.Sprintf("\n-%v**:**`%v`**.**", i, guildTopics.Topics[topicname].Triggers)
				}
				session.ChannelMessageSend(message.ChannelID, str)
			}
		}
	}
	config.Guild[guildID] = guildTopics
}

var japeFlavourText = []string{
	"You went too far, you fool.",
	"Awe heck, I can't believe you've done this.",
	"OP is a coward.",
	"Think before you speak.",
	"You made a stupid post so now you get no rights.",
	"Do you value your toes?",
	"Arrested for stupid on main.",
	"You'll recieve your L in 5-8 buisness days.",
}

// If matchedTopic is called, it is already assumed the regex has been met.
// Regex will not be checked.
func matchedTopic(t topic, s *dsg.Session, m *dsg.Message) topic {
	now := time.Now()
	if triggerReply[m.ChannelID].MessageCount < 120 && now.Sub(triggerReply[m.ChannelID].LastTrigger) < 5*time.Minute {
		if time.Since(t.CurrentStreakSet) > t.HighScoreBroken.Sub(t.HighScoreSet) {
			t.HighScoreSetter = t.LastStreakSetter
			t.HighScoreSet = t.LastStreakSet
			t.LastStreakSetter = m.Author.ID
			t.HighScoreBreaker = m.Author.ID
			t.LastStreakSet = now
			t.HighScoreBroken = now
			t.Message = m.Content
			t.CurrentStreakSet = now
			if tmp, _ := triggerReply[m.ChannelID]; true {
				tmp.MessageCount = 0
				tmp.LastTrigger = now
				triggerReply[m.ChannelID] = tmp
			}
			return t
		} else {
			t.LastStreakSetter = m.Author.ID
			t.LastStreakSet = now
			t.CurrentStreakSet = now
			if tmp, _ := triggerReply[m.ChannelID]; true {
				tmp.MessageCount = 0
				tmp.LastTrigger = now
				triggerReply[m.ChannelID] = tmp
			}
			return t
		}
	}

	rand.Seed(now.Unix())
	if time.Since(t.CurrentStreakSet) > t.HighScoreBroken.Sub(t.HighScoreSet) {
		s.ChannelMessageSendEmbed(m.ChannelID, &dsg.MessageEmbed{
			Author:      &dsg.MessageEmbedAuthor{},
			Color:       0x2AA198,
			Title:       fmt.Sprintf("**New Record!** %v", japeFlavourText[rand.Intn(len(japeFlavourText))]),
			Description: fmt.Sprintf("%v has broken the %v streak.", m.Author.Mention(), t.Name),
			Thumbnail: &dsg.MessageEmbedThumbnail{
				URL:    fmt.Sprintf("https://cdn.whitmans.io/streakreactions/newstreak.png"),
				Width:  64,
				Height: 64,
			},
			Fields: []*dsg.MessageEmbedField{
				&dsg.MessageEmbedField{
					Name: "Previous Longest Streak",
					Value: fmt.Sprintf("Set by <@%v> at %v, and broken by <@%v> at %v (%v) with the messsage\n```%v```",
						t.HighScoreSetter,
						t.HighScoreSet.Format("Mon, Jan 2 2006 at 15:04 (MST)"),
						t.HighScoreBreaker,
						t.HighScoreBroken.Format("Mon, Jan 2 2006 at 15:04 (MST)"),
						t.HighScoreBroken.Sub(t.HighScoreSet).Truncate(time.Second),
						t.Message),
					Inline: true,
				},
				&dsg.MessageEmbedField{
					Name: "New Longest Streak",
					Value: fmt.Sprintf("Set by <@%v> at %v, and broken by %v at %v (%v) with the message\n```%v```",
						t.LastStreakSetter,
						t.LastStreakSet.Format("Mon, Jan 2 2006 at 15:04 (MST)"),
						m.Author.Mention(),
						now.Format("Mon, Jan 2 2006 at 15:04 (MST)"),
						time.Since(t.LastStreakSet).Truncate(time.Second),
						m.Content),
					Inline: true,
				},
			},
		})
		t.HighScoreSetter = t.LastStreakSetter
		t.HighScoreSet = t.LastStreakSet
		t.LastStreakSetter = m.Author.ID
		t.HighScoreBreaker = m.Author.ID
		t.LastStreakSet = now
		t.HighScoreBroken = now
		t.Message = m.Content
		t.CurrentStreakSet = now
		return t
	} else {
		s.ChannelMessageSendEmbed(m.ChannelID, &dsg.MessageEmbed{
			Author:      &dsg.MessageEmbedAuthor{},
			Color:       0xDC322F,
			Title:       fmt.Sprintf("**Streak Broken.** %v", japeFlavourText[rand.Intn(len(japeFlavourText))]),
			Description: fmt.Sprintf("%v has broken the %v streak.", m.Author.Mention(), t.Name),
			Thumbnail: &dsg.MessageEmbedThumbnail{
				URL:    fmt.Sprintf("https://cdn.whitmans.io/streakreactions/streakbroken.png"),
				Width:  64,
				Height: 64,
			},
			Fields: []*dsg.MessageEmbedField{
				&dsg.MessageEmbedField{
					Name: "Highest Streak",
					Value: fmt.Sprintf("Set by <@%v> at %v, and broken by <@%v> at %v (%v) with the messsage\n```%v```",
						t.HighScoreSetter,
						t.HighScoreSet.Format("Mon, Jan 2 2006 at 15:04 (MST)"),
						t.HighScoreBreaker,
						t.HighScoreBroken.Format("Mon, Jan 2 2006 at 15:04 (MST)"),
						t.HighScoreBroken.Sub(t.HighScoreSet).Truncate(time.Second),
						t.Message),
					Inline: true,
				},
				&dsg.MessageEmbedField{
					Name: "Current Streak",
					Value: fmt.Sprintf("Set by <@%v> at %v, and broken by %v at %v (%v) with the message\n```%v```",
						t.LastStreakSetter,
						t.LastStreakSet.Format("Mon, Jan 2 2006 at 15:04 (MST)"),
						m.Author.Mention(),
						now.Format("Mon, Jan 2 2006 at 15:04 (MST)"),
						time.Since(t.LastStreakSet).Truncate(time.Second),
						m.Content),
					Inline: true,
				},
			},
		})
		t.LastStreakSetter = m.Author.ID
		t.LastStreakSet = now
		t.CurrentStreakSet = now
		return t
	}
}
