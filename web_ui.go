package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"github.com/adhocore/gronx"
	"github.com/charmbracelet/log"
	"github.com/gin-contrib/multitemplate"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	bolt "go.etcd.io/bbolt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"strconv"
	"time"
)

type WebToken struct {
	Token     string
	CreatedAt time.Time
	Channel   string
}

type webUI struct {
	tokens []WebToken
}

//go:embed static/*
var staticFiles embed.FS

//go:embed templates/*
var templateFiles embed.FS

func ServeUI() {
	defer App.wg.Done()

	ui := webUI{}
	App.webUI = &ui
	r := gin.New()
	r.Use(gin.Recovery())

	if !App.config.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	r.HTMLRender = ui.newRenderer()
	staticFs, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Error("Cannot load static files.", "err", err)
	}
	r.StaticFS("/static/", http.FS(staticFs))

	g := r.Group("/:channel/:token/", ui.checkToken)
	g.GET("/", ui.handleQuestionList)
	g.GET("/new/", ui.handleNewQuestion)
	g.POST("/new/", ui.handleNewQuestionPost)
	g.GET("/edit/:id/", ui.handleEditQuestion)
	g.POST("/edit/:id/", ui.handleEditQuestionPost)
	g.POST("/invoke/:id/", ui.handleInvokeQuestion)

	err = r.Run(App.config.ListenAddress)
	if err != nil {
		log.Error("Error while running Web UI.", "err", err)
	}
}

func (w *webUI) CreateToken(channel string) string {
	token := WebToken{
		Token:     uuid.NewString(),
		CreatedAt: time.Now(),
		Channel:   channel,
	}

	w.tokens = append(w.tokens, token)
	return token.Token
}

func (w *webUI) checkToken(ctx *gin.Context) {
	token := ctx.Param("token")
	channel := ctx.Param("channel")

	var goodTokens []WebToken
	ok := false
	for _, webToken := range w.tokens {
		if webToken.CreatedAt.Add(1 * time.Hour).Before(time.Now()) {
			continue
		}
		goodTokens = append(goodTokens, webToken)
		if webToken.Token == token && webToken.Channel == channel {
			ok = true
		}
	}

	if !ok {
		ctx.String(http.StatusForbidden, "Invalid access token.")
		ctx.Abort()
		return
	}
	ctx.Next()
}

func (w *webUI) createTemplate(files ...string) *template.Template {
	tmpl, err := template.ParseFS(templateFiles, files...)
	if err != nil {
		log.Error("Could not parse template.", "err", err)
		os.Exit(1)
	}
	return tmpl
}

func (w *webUI) newRenderer() multitemplate.Renderer {
	r := multitemplate.NewRenderer()
	r.Add("question_list", w.createTemplate("templates/base.gohtml", "templates/question_list.gohtml"))
	r.Add("question_form", w.createTemplate("templates/base.gohtml", "templates/question_form.gohtml"))
	return r
}

func (w *webUI) error(ctx *gin.Context, err error) {
	log.Error("Error during HTTP request.", "request", ctx.Request.URL.Path, "method", ctx.Request.Method, "err", err)
	ctx.String(http.StatusInternalServerError, "Server Error")
	ctx.Abort()
}

func (w *webUI) render(ctx *gin.Context, template string, context gin.H) {
	context["URLPrefix"] = fmt.Sprintf("/%s/%s", ctx.Param("channel"), ctx.Param("token"))
	ctx.HTML(http.StatusOK, template, context)
}

type userInfo struct {
	ID       string
	Name     string
	Selected bool
}

func (w *webUI) listChannelMembers(channel string) ([]userInfo, error) {
	users, err := ListChannelMembers(channel)
	if err != nil {
		return nil, err
	}
	var userInfos []userInfo

	for _, user := range users {
		if user == App.myUserId {
			continue
		}

		name, err := LoadMemberName(user)
		if err != nil {
			return nil, err
		}

		userInfos = append(userInfos, userInfo{
			ID:   user,
			Name: name,
		})
	}

	return userInfos, nil
}

func (w *webUI) handleQuestionList(ctx *gin.Context) {
	var questions []Question

	err := App.db.View(func(tx *bolt.Tx) error {
		return tx.Bucket([]byte("questions")).ForEach(func(k, v []byte) error {
			var q Question
			err := json.Unmarshal(v, &q)
			if err != nil {
				return err
			}

			questions = append(questions, q)
			return nil
		})
	})
	if err != nil {
		w.error(ctx, fmt.Errorf("cannot load questions: %w", err))
		return
	}

	w.render(ctx, "question_list", gin.H{"questions": questions})
}

func (w *webUI) handleNewQuestion(ctx *gin.Context) {
	users, err := w.listChannelMembers(ctx.Param("channel"))
	if err != nil {
		w.error(ctx, fmt.Errorf("could not get channel members: %w", err))
		return
	}

	w.render(ctx, "question_form", gin.H{"users": users})
}

type questionForm struct {
	Users   []string `binding:"required" form:"users"`
	Message string   `binding:"required" form:"message"`
	Cron    string   `binding:"required" form:"cron"`
	Active  bool     `form:"active"`
}

func (w *webUI) handleNewQuestionPost(ctx *gin.Context) {
	var data questionForm
	err := ctx.Bind(&data)
	if err != nil {
		ctx.String(400, "Invalid form data.")
		return
	}

	gron := gronx.New()
	valid := gron.IsValid(data.Cron)
	if !valid {
		ctx.String(http.StatusBadRequest, "Invalid cron expression.")
		return
	}

	question := Question{
		Channel:         ctx.Param("channel"),
		Message:         data.Message,
		Users:           data.Users,
		Cron:            data.Cron,
		CurrentInstance: "",
		IsActive:        data.Active,
	}
	err = question.Save()
	if err != nil {
		ctx.String(http.StatusInternalServerError, "Server Error")
		return
	}

	ctx.Redirect(http.StatusFound, fmt.Sprintf("/%s/%s/edit/%d/", ctx.Param("channel"), ctx.Param("token"), question.ID))
}

func (w *webUI) handleEditQuestion(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.String(http.StatusNotFound, "Not found")
		return
	}

	question, err := LoadQuestion(id)
	if err != nil || question.Channel != ctx.Param("channel") {
		ctx.String(http.StatusNotFound, "Not found")
		return
	}

	users, err := w.listChannelMembers(question.Channel)
	if err != nil {
		w.error(ctx, fmt.Errorf("could not get channel members: %w", err))
		return
	}

	for i, user := range users {
		selected := false
		for _, s := range question.Users {
			if user.ID == s {
				selected = true
				break
			}
		}
		users[i].Selected = selected
	}

	w.render(ctx, "question_form", gin.H{"users": users, "question": question})
}

func (w *webUI) handleEditQuestionPost(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.String(http.StatusNotFound, "Not found")
		return
	}

	question, err := LoadQuestion(id)
	if err != nil || question.Channel != ctx.Param("channel") {
		ctx.String(http.StatusNotFound, "Not found")
		return
	}

	var data questionForm
	err = ctx.Bind(&data)
	if err != nil {
		ctx.String(400, "Invalid form data.")
		return
	}

	gron := gronx.New()
	valid := gron.IsValid(data.Cron)
	if !valid {
		ctx.String(http.StatusBadRequest, "Invalid cron expression.")
		return
	}

	question.Message = data.Message
	question.Users = data.Users
	question.Cron = data.Cron
	question.IsActive = data.Active
	err = question.Save()
	if err != nil {
		w.error(ctx, fmt.Errorf("could not save question: %w", err))
		return
	}

	ctx.Redirect(http.StatusFound, fmt.Sprintf("/%s/%s/", ctx.Param("channel"), ctx.Param("token")))
}

func (w *webUI) handleInvokeQuestion(ctx *gin.Context) {
	id, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.String(http.StatusNotFound, "Not found")
		return
	}

	question, err := LoadQuestion(id)
	if err != nil || question.Channel != ctx.Param("channel") {
		ctx.String(http.StatusNotFound, "Not found")
		return
	}

	err = question.NewInstance()
	if err != nil {
		w.error(ctx, fmt.Errorf("could not invoke question: %w", err))
		return
	}

	ctx.Redirect(http.StatusFound, fmt.Sprintf("/%s/%s/", ctx.Param("channel"), ctx.Param("token")))
}
