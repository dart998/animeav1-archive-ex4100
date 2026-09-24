package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	adminCookieName = "animeav1_admin_session"
	adminSessionDuration = 30 * 24 * time.Hour
	adminPasswordIterations = 80000
)

type adminAuthData struct {
	Setup bool
	Error string
	Next string
}

func (s *Server) adminConfigured() bool {
	return strings.TrimSpace(s.db.GetSetting("admin_username"))!="" &&
		strings.TrimSpace(s.db.GetSetting("admin_password_salt"))!="" &&
		strings.TrimSpace(s.db.GetSetting("admin_password_hash"))!="" &&
		strings.TrimSpace(s.db.GetSetting("admin_session_secret"))!=""
}

func randomBytes(n int)([]byte,error){b:=make([]byte,n);_,e:=rand.Read(b);return b,e}

func pbkdf2SHA256(password string,salt []byte,iterations int)[]byte{
	if iterations<1{iterations=1}
	mac:=hmac.New(sha256.New,[]byte(password))
	block:=make([]byte,len(salt)+4);copy(block,salt);binary.BigEndian.PutUint32(block[len(salt):],1)
	mac.Write(block);u:=mac.Sum(nil);out:=append([]byte(nil),u...)
	for i:=1;i<iterations;i++{mac=hmac.New(sha256.New,[]byte(password));mac.Write(u);u=mac.Sum(nil);for j:=range out{out[j]^=u[j]}}
	return out
}

func (s *Server) configureAdmin(username,password string) error {
	username=strings.TrimSpace(username)
	if len(username)<3||len(username)>64{return errors.New("el usuario debe tener entre 3 y 64 caracteres")}
	if strings.Contains(username,"|"){return errors.New("el usuario contiene un carácter no permitido")}
	if len(password)<8{return errors.New("la contraseña debe tener al menos 8 caracteres")}
	salt,e:=randomBytes(16);if e!=nil{return e};secret,e:=randomBytes(32);if e!=nil{return e}
	hash:=pbkdf2SHA256(password,salt,adminPasswordIterations)
	tx,e:=s.db.Begin();if e!=nil{return e};defer tx.Rollback()
	values:=map[string]string{
		"admin_username":username,
		"admin_password_salt":base64.RawURLEncoding.EncodeToString(salt),
		"admin_password_hash":base64.RawURLEncoding.EncodeToString(hash),
		"admin_session_secret":base64.RawURLEncoding.EncodeToString(secret),
	}
	for k,v:=range values{if _,e=tx.Exec(`INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,k,v);e!=nil{return e}}
	return tx.Commit()
}

func (s *Server) verifyAdminPassword(username,password string) bool {
	if !s.adminConfigured()||strings.TrimSpace(username)!=strings.TrimSpace(s.db.GetSetting("admin_username")){return false}
	salt,e:=base64.RawURLEncoding.DecodeString(s.db.GetSetting("admin_password_salt"));if e!=nil{return false}
	want,e:=base64.RawURLEncoding.DecodeString(s.db.GetSetting("admin_password_hash"));if e!=nil{return false}
	got:=pbkdf2SHA256(password,salt,adminPasswordIterations)
	return hmac.Equal(got,want)
}

func (s *Server) adminSessionSecret()([]byte,error){
	secret,e:=base64.RawURLEncoding.DecodeString(s.db.GetSetting("admin_session_secret"));if e!=nil||len(secret)<32{return nil,errors.New("secreto de sesión inválido")};return secret,nil
}

func (s *Server) newAdminSessionToken(now time.Time)(string,error){
	secret,e:=s.adminSessionSecret();if e!=nil{return "",e}
	username:=strings.TrimSpace(s.db.GetSetting("admin_username"));exp:=now.Add(adminSessionDuration).Unix()
	payload:=username+"|"+strconv.FormatInt(exp,10);enc:=base64.RawURLEncoding.EncodeToString([]byte(payload))
	mac:=hmac.New(sha256.New,secret);mac.Write([]byte(enc));sig:=base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return enc+"."+sig,nil
}

func (s *Server) validAdminSession(token string,now time.Time) bool {
	parts:=strings.Split(token,".");if len(parts)!=2{return false}
	secret,e:=s.adminSessionSecret();if e!=nil{return false}
	mac:=hmac.New(sha256.New,secret);mac.Write([]byte(parts[0]));got,e:=base64.RawURLEncoding.DecodeString(parts[1]);if e!=nil||!hmac.Equal(got,mac.Sum(nil)){return false}
	raw,e:=base64.RawURLEncoding.DecodeString(parts[0]);if e!=nil{return false};fields:=strings.Split(string(raw),"|");if len(fields)!=2{return false}
	if fields[0]!=strings.TrimSpace(s.db.GetSetting("admin_username")){return false};exp,e:=strconv.ParseInt(fields[1],10,64);return e==nil&&now.Unix()<exp
}

func adminCookieSecure(r *http.Request)bool{return r.TLS!=nil||strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")),"https")}

func (s *Server) setAdminSession(w http.ResponseWriter,r *http.Request) error {
	token,e:=s.newAdminSessionToken(time.Now());if e!=nil{return e}
	http.SetCookie(w,&http.Cookie{Name:adminCookieName,Value:token,Path:"/admin",HttpOnly:true,Secure:adminCookieSecure(r),SameSite:http.SameSiteStrictMode,MaxAge:int(adminSessionDuration.Seconds()),Expires:time.Now().Add(adminSessionDuration)})
	return nil
}

func clearAdminSession(w http.ResponseWriter,r *http.Request){
	http.SetCookie(w,&http.Cookie{Name:adminCookieName,Value:"",Path:"/admin",HttpOnly:true,Secure:adminCookieSecure(r),SameSite:http.SameSiteStrictMode,MaxAge:-1,Expires:time.Unix(1,0)})
}

func (s *Server) hasValidAdminSession(r *http.Request)bool{c,e:=r.Cookie(adminCookieName);return e==nil&&s.validAdminSession(c.Value,time.Now())}

func cleanAdminNext(raw string)string{raw=strings.TrimSpace(raw);if raw==""||!strings.HasPrefix(raw,"/admin")||strings.HasPrefix(raw,"//"){return "/admin"};return raw}

func (s *Server) requireAdmin(next http.HandlerFunc)http.HandlerFunc{
	return func(w http.ResponseWriter,r *http.Request){
		if !s.adminConfigured(){http.Redirect(w,r,"/admin/setup",http.StatusSeeOther);return}
		if s.hasValidAdminSession(r){next(w,r);return}
		target:="/admin";if r.Method==http.MethodGet||r.Method==http.MethodHead{target=cleanAdminNext(r.URL.RequestURI())}
		http.Redirect(w,r,"/admin/login?next="+url.QueryEscape(target),http.StatusSeeOther)
	}
}

func (s *Server) renderAdminAuth(w http.ResponseWriter,data adminAuthData){
	w.Header().Set("Cache-Control","no-store");if e:=s.tmpl.ExecuteTemplate(w,"admin_auth.html",data);e!=nil{http.Error(w,e.Error(),500)}
}

func (s *Server) adminSetup(w http.ResponseWriter,r *http.Request){
	if s.adminConfigured(){http.Redirect(w,r,"/admin",http.StatusSeeOther);return}
	if r.Method==http.MethodGet{s.renderAdminAuth(w,adminAuthData{Setup:true});return}
	if r.Method!=http.MethodPost{http.Error(w,"method not allowed",405);return}
	if e:=r.ParseForm();e!=nil{http.Error(w,e.Error(),400);return}
	user:=strings.TrimSpace(r.FormValue("username"));password:=r.FormValue("password");confirm:=r.FormValue("confirm_password")
	if password!=confirm{s.renderAdminAuth(w,adminAuthData{Setup:true,Error:"Las contraseñas no coinciden."});return}
	if e:=s.configureAdmin(user,password);e!=nil{s.renderAdminAuth(w,adminAuthData{Setup:true,Error:e.Error()});return}
	if e:=s.setAdminSession(w,r);e!=nil{http.Error(w,e.Error(),500);return};http.Redirect(w,r,"/admin",http.StatusSeeOther)
}

func (s *Server) adminLogin(w http.ResponseWriter,r *http.Request){
	if !s.adminConfigured(){http.Redirect(w,r,"/admin/setup",http.StatusSeeOther);return}
	next:=cleanAdminNext(r.URL.Query().Get("next"));if r.Method==http.MethodGet{if s.hasValidAdminSession(r){http.Redirect(w,r,next,http.StatusSeeOther);return};s.renderAdminAuth(w,adminAuthData{Next:next});return}
	if r.Method!=http.MethodPost{http.Error(w,"method not allowed",405);return};if e:=r.ParseForm();e!=nil{http.Error(w,e.Error(),400);return};next=cleanAdminNext(r.FormValue("next"))
	if !s.verifyAdminPassword(r.FormValue("username"),r.FormValue("password")){s.renderAdminAuth(w,adminAuthData{Error:"Usuario o contraseña incorrectos.",Next:next});return}
	if e:=s.setAdminSession(w,r);e!=nil{http.Error(w,e.Error(),500);return};http.Redirect(w,r,next,http.StatusSeeOther)
}

func (s *Server) adminLogout(w http.ResponseWriter,r *http.Request){
	if r.Method!=http.MethodPost{http.Error(w,"method not allowed",405);return};clearAdminSession(w,r);http.Redirect(w,r,"/admin/login",http.StatusSeeOther)
}
