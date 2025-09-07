package boardwhite

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/boar-d-white-foundation/drone/db"
	"github.com/boar-d-white-foundation/drone/tg"
	tele "gopkg.in/telebot.v3"
)

const meetupAddUnique = "meetup_add"

type meetupStore map[string][]tele.User

func addUserToMeetup(tx db.Tx, location string, user tele.User) error {
	meetups, err := db.GetJsonDefault(tx, keyMeetupLocations, meetupStore{})
	if err != nil {
		return fmt.Errorf("get meetups: %w", err)
	}
	users := meetups[location]
	if idx := slices.IndexFunc(users, func(u tele.User) bool { return u.ID == user.ID }); idx == -1 {
		meetups[location] = append(users, user)
	}
	if err := db.SetJson(tx, keyMeetupLocations, meetups); err != nil {
		return fmt.Errorf("set meetups: %w", err)
	}
	return nil
}

func listMeetupLocations(tx db.Tx) ([]string, error) {
	meetups, err := db.GetJsonDefault(tx, keyMeetupLocations, meetupStore{})
	if err != nil {
		return nil, fmt.Errorf("get meetups: %w", err)
	}
	locations := make([]string, 0, len(meetups))
	for loc := range meetups {
		locations = append(locations, loc)
	}
	slices.Sort(locations)
	return locations, nil
}

func (s *Service) OnMeetup(ctx context.Context, c tele.Context) error {
	msg, chat := c.Message(), c.Chat()
	if msg == nil || chat == nil || chat.ID != s.cfg.ChatID {
		return nil
	}
	if !strings.HasPrefix(msg.Text, "/сходка") {
		return nil
	}

	parts := strings.Fields(msg.Text)
	if len(parts) < 2 || parts[1] == "list" {
		return s.database.Do(ctx, func(tx db.Tx) error {
			locations, err := listMeetupLocations(tx)
			if err != nil {
				return err
			}
			if len(locations) == 0 {
				_, err = s.telegram.ReplyWithText(msg.ID, "Пока нет локаций")
				return err
			}
			_, err = s.telegram.ReplyWithText(msg.ID, strings.Join(locations, "\n"))
			return err
		})
	}
	if parts[1] == "add" {
		if len(parts) < 3 {
			return s.database.Do(ctx, func(tx db.Tx) error {
				meetups, err := db.GetJsonDefault(tx, keyMeetupLocations, meetupStore{})
				if err != nil {
					return fmt.Errorf("get meetups: %w", err)
				}
				if len(meetups) == 0 {
					_, err = s.telegram.ReplyWithText(msg.ID, "Пока нет локаций, добавь новую через /сходка add {локация}")
					return err
				}
				markup := &tele.ReplyMarkup{}
				rows := make([]tele.Row, 0, len(meetups))
				for loc := range meetups {
					btn := markup.Data(loc, meetupAddUnique, loc)
					rows = append(rows, markup.Row(btn))
				}
				markup.Inline(rows...)
				_, err = s.telegram.SendTextWithMarkup(msg.ThreadID, "Выбери локацию", markup)
				return err
			})
		}
		location := strings.Join(parts[2:], " ")
		if msg.Sender == nil {
			return nil
		}
		err := s.database.Do(ctx, func(tx db.Tx) error {
			return addUserToMeetup(tx, location, *msg.Sender)
		})
		if err != nil {
			return err
		}
		mention := tg.BuildMentionMarkdownV2(*msg.Sender)
		_, err = s.telegram.SendMarkdownV2(msg.ThreadID, fmt.Sprintf("%s добавлен в %s", mention, tg.EscapeMD(location)))
		return err
	}

	location := strings.Join(parts[1:], " ")
	return s.database.Do(ctx, func(tx db.Tx) error {
		meetups, err := db.GetJsonDefault(tx, keyMeetupLocations, meetupStore{})
		if err != nil {
			return fmt.Errorf("get meetups: %w", err)
		}
		users := meetups[location]
		if len(users) == 0 {
			_, err = s.telegram.ReplyWithText(msg.ID, "В этой локации никого нет")
			return err
		}
		mentions := make([]string, 0, len(users))
		for _, u := range users {
			mentions = append(mentions, tg.BuildMentionMarkdownV2(u))
		}
		text := strings.Join(mentions, " ")
		_, err = s.telegram.SendMarkdownV2(msg.ThreadID, text)
		return err
	})
}

func (s *Service) OnMeetupCallback(ctx context.Context, c tele.Context) error {
	cb, msg, chat := c.Callback(), c.Message(), c.Chat()
	if cb == nil || msg == nil || chat == nil || chat.ID != s.cfg.ChatID {
		return nil
	}
	if cb.Unique != meetupAddUnique {
		return nil
	}
	if cb.Sender == nil {
		return nil
	}
	location := cb.Data
	err := s.database.Do(ctx, func(tx db.Tx) error {
		return addUserToMeetup(tx, location, *cb.Sender)
	})
	if err != nil {
		return err
	}
	mention := tg.BuildMentionMarkdownV2(*cb.Sender)
	_, err = s.telegram.SendMarkdownV2(msg.ThreadID, fmt.Sprintf("%s добавлен в %s", mention, tg.EscapeMD(location)))
	if err != nil {
		return err
	}
	return c.Respond()
}
