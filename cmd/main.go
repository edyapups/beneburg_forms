package main

import (
	"encoding/json"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
)

const (
	AdminIdEnvName = "BENEBURG_ADMIN_ID"
	TokenEnvName   = "BENEBURG_BOT_TOKEN"
	PortEnvName    = "BENEBURG_PORT"
	FormUrlEnvName = "BENEBURG_FORM_URL"
)

func main() {
	// Logging
	logger, _ := zap.NewDevelopment()
	undo := zap.ReplaceGlobals(logger)
	defer undo()

	// Getting configuration
	var token, port, formUrl string
	var adminId int64
	adminId, err := strconv.ParseInt(os.Getenv(AdminIdEnvName), 10, 64)
	if err != nil {
		zap.S().Fatalf("%s is not specified or is not a valid int64 value", AdminIdEnvName)
	}
	token = os.Getenv(TokenEnvName)
	if len(token) == 0 {
		zap.S().Fatalf("%s must be specified", TokenEnvName)
	}
	formUrl = os.Getenv(FormUrlEnvName)
	if len(formUrl) == 0 {
		zap.S().Fatalf("%s must be specified", FormUrlEnvName)
	}
	port = os.Getenv(PortEnvName)
	if len(port) == 0 {
		zap.S().Infof("%s is not specified, using default value 3333", PortEnvName)
		port = "3333"
	}

	// URL Generator
	urlGenerator, err := getUrlGenerator(formUrl)
	if err != nil {
		zap.S().Fatal(err)
	}

	// Bot initiation
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		zap.S().Fatal(err)
	}

	// Start handling form responses
	go func() {
		http.HandleFunc(
			"/form",
			makeFormHandler(bot, adminId),
		)
		err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil)
		zap.S().Info(err)
		notifyError(bot, adminId, err)
	}()

	// Start handling bot updates
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30
	updates := bot.GetUpdatesChan(updateConfig)
	for update := range updates {
		if update.Message == nil || update.Message.Chat == nil {
			continue
		}

		zap.S().Debug("New message from ", update.Message.From)

		user := update.Message.From
		if update.Message.ForwardFrom != nil {
			user = update.Message.ForwardFrom
		}

		text := getFormUrlText(user, urlGenerator)
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, text)
		msg.ParseMode = tgbotapi.ModeHTML
		msg.ReplyToMessageID = update.Message.MessageID
		msg.AllowSendingWithoutReply = true
		msg.DisableWebPagePreview = true
		if _, err = bot.Send(msg); err != nil {
			notifyError(bot, adminId, err)
			zap.S().Error(err)
		}

		if update.Message.Chat.ID == adminId {
			continue
		}
		msg = tgbotapi.NewMessage(adminId, text)
		if _, err = bot.Send(msg); err != nil {
			notifyError(bot, adminId, err)
			zap.S().Error(err)
		}
	}
}

func makeFormHandler(bot *tgbotapi.BotAPI, adminId int64) func(http.ResponseWriter, *http.Request) {
	return func(writer http.ResponseWriter, request *http.Request) {
		data, err := io.ReadAll(request.Body)
		if err != nil {
			zap.S().Error(err)
			notifyError(bot, adminId, err)
		}
		form, err := marshalResponse(data)
		if err != nil {
			zap.S().Error(err, string(data))
			notifyError(bot, adminId, err)
		}
		zap.S().Debug("Got form ", form)
		text := getFormTextResult(form)
		msg := tgbotapi.NewMessage(form.UserId.Int64(), "Отлично, мы получили твою анкету. Вот как она выглядит:\n\n"+text)
		msg.ParseMode = tgbotapi.ModeHTML
		if _, err = bot.Send(msg); err != nil {
			zap.S().Error(err)
		}
		msg.Text = text
		msg.BaseChat.ChatID = adminId
		if _, err = bot.Send(msg); err != nil {
			zap.S().Error(err)
		}
		zap.S().Debug("Sent message ", msg.ChatID)
	}
}

func notifyError(bot *tgbotapi.BotAPI, adminId int64, err error) {
	msg := tgbotapi.NewMessage(adminId, err.Error())
	_, _ = bot.Send(msg)
}

type FormData struct {
	Name      string  `json:"name"`
	Age       int     `json:"age"`
	About     string  `json:"about,omitempty"`
	Hobby     string  `json:"hobby,omitempty"`
	Work      string  `json:"work,omitempty"`
	Education string  `json:"education,omitempty"`
	Why       string  `json:"why,omitempty"`
	Agree     string  `json:"agree"`
	Vote      string  `json:"vote"`
	UserId    big.Int `json:"user_id"`
}

func marshalResponse(data []byte) (*FormData, error) {
	var resp FormData
	err := json.Unmarshal(data, &resp)
	if err != nil {
		return nil, err
	}
	id, err := admitID(resp.UserId.String())
	if err != nil {
		return nil, err
	}
	resp.UserId.SetInt64(id)
	return &resp, nil
}

func getFormTextResult(form *FormData) string {
	result := strings.Builder{}

	result.WriteString(fmt.Sprintf(
		"<b><a href=\"tg://user?id=%s\">Профиль</a></b>\n",
		form.UserId.String(),
	))

	// Name
	result.WriteString(fmt.Sprintf("\n<b>Как обращаться:</b>\n%s", tgbotapi.EscapeText(tgbotapi.ModeHTML, form.Name)))

	// Age
	result.WriteString(fmt.Sprintf("\n\n<b>Возраст:</b>\n%d", form.Age))

	// About
	if form.About != "" {
		result.WriteString(fmt.Sprintf(
			"\n\n<b>О себе, своем характере, социальных, политических и других интересных взглядах:</b>\n%s",
			tgbotapi.EscapeText(tgbotapi.ModeHTML, form.About),
		))
	}

	// Hobby
	if form.Hobby != "" {
		result.WriteString(fmt.Sprintf(
			"\n\n<b>О своих хобби:</b>\n%s",
			tgbotapi.EscapeText(tgbotapi.ModeHTML, form.Hobby),
		))
	}

	// Work
	if form.Work != "" {
		result.WriteString(fmt.Sprintf(
			"\n\n<b>Работа:</b>\n%s",
			tgbotapi.EscapeText(tgbotapi.ModeHTML, form.Work),
		))
	}

	// Education
	if form.Education != "" {
		result.WriteString(fmt.Sprintf(
			"\n\n<b>Образование:</b>\n%s",
			tgbotapi.EscapeText(tgbotapi.ModeHTML, form.Education),
		))
	}

	// Why
	if form.Why != "" {
		result.WriteString(fmt.Sprintf(
			"\n\n<b>Почему хочет в чатик:</b>\n%s",
			tgbotapi.EscapeText(tgbotapi.ModeHTML, form.Why),
		))
	}

	// Agree
	result.WriteString(fmt.Sprintf(
		"\n\n<b>Согласен_а на пересылку анкеты в чатик:</b>\n%s",
		tgbotapi.EscapeText(tgbotapi.ModeHTML, form.Agree),
	))

	// Vote
	result.WriteString(fmt.Sprintf(
		"\n\n<b>Понравилось заполнять анкетку?</b>\n%s",
		tgbotapi.EscapeText(tgbotapi.ModeHTML, form.Vote),
	))

	// ID
	result.WriteString(fmt.Sprintf(
		"\n\n<i>ID пользователя: </i><code>%s</code>",
		form.UserId.String(),
	))

	result.WriteString("\n#анкета")
	return result.String()
}

func getFormUrlText(user *tgbotapi.User, urlGenerator func(userID int64) string) string {
	result := strings.Builder{}
	result.WriteString(fmt.Sprintf(
		"Привет, <b>%s</b>!\n",
		tgbotapi.EscapeText(tgbotapi.ModeHTML, user.FirstName),
	))
	personalUrl := urlGenerator(user.ID)
	result.WriteString(fmt.Sprintf(
		"<a href=\"%s\">Вот твоя ссылка на форму новичков.</a>",
		personalUrl,
	))
	return result.String()
}

func getUrlGenerator(formUrl string) (func(userID int64) string, error) {
	parsedUrl, err := url.Parse(formUrl)
	if err != nil {
		return nil, err
	}
	query := parsedUrl.Query()
	return func(userID int64) string {
		query.Set("user_id", hideID(userID))
		parsedUrl.RawQuery = query.Encode()
		return parsedUrl.String()
	}, nil
}

// ID Hiding

const idMultiplier = 55103465

func hideID(id int64) string {
	bigId := new(big.Int)
	bigId.SetString(doubleString(strconv.FormatInt(id, 10)), 10)
	bigId.Mul(bigId, big.NewInt(idMultiplier))
	return bigId.String()
}
func admitID(id string) (int64, error) {
	bigId := new(big.Int)
	_, ok := bigId.SetString(id, 10)
	if !ok {
		return 0, fmt.Errorf("%s is not valid integer", id)
	}
	_, mod := bigId.DivMod(bigId, big.NewInt(idMultiplier), new(big.Int))
	if mod.Cmp(big.NewInt(0)) != 0 {
		return 0, fmt.Errorf("%s is not valid id", id)
	}
	result, err := strconv.ParseInt(deDoubleString(bigId.String()), 10, 64)
	if err != nil {
		return 0, err
	}
	return result, nil
}

func doubleString(str string) string {
	result := strings.Builder{}
	for _, char := range str {
		result.WriteRune(char)
		result.WriteRune(char)
	}
	return result.String()
}

func deDoubleString(str string) string {
	result := strings.Builder{}
	for index, char := range str {
		if index%2 == 0 {
			result.WriteRune(char)
		}
	}
	return result.String()
}
