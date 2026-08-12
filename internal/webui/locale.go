package webui

import "net/url"

type locale string

const (
	localeJA locale = "ja"
	localeEN locale = "en"
)

func (l locale) lang() string {
	return string(l)
}

func localizedPath(l locale, localPath string) string {
	if localPath == "" || localPath[0] != '/' {
		localPath = "/" + localPath
	}
	if l == localeEN {
		if localPath == "/" {
			return "/en/"
		}
		return "/en" + localPath
	}
	return localPath
}

func entryHref(l locale, id string) string {
	return localizedPath(l, "/entries/"+url.PathEscape(id))
}

func runHref(l locale, id string) string {
	return localizedPath(l, "/imports/"+url.PathEscape(id))
}

func entryRevisionHref(l locale, id string) string {
	return localizedPath(l, "/ui/entries/"+url.PathEscape(id)+"/revisions")
}

func entryApprovalHref(l locale, id string) string {
	return localizedPath(l, "/ui/entries/"+url.PathEscape(id)+"/approvals")
}

func importHref(l locale) string {
	return localizedPath(l, "/ui/imports")
}

func searchHref(l locale) string {
	return localizedPath(l, "/ui/entries/search")
}

func exportHref(l locale, format string) string {
	return localizedPath(l, "/ui/exports/"+url.PathEscape(format))
}

type pageContext struct {
	Locale        locale
	Messages      messages
	HomeHref      string
	JapaneseHref  string
	EnglishHref   string
	CurrentLocale string
}

func newPageContext(l locale, localPath string) pageContext {
	msg := messagesFor(l)
	return pageContext{
		Locale:        l,
		Messages:      msg,
		HomeHref:      localizedPath(l, "/"),
		JapaneseHref:  localizedPath(localeJA, localPath),
		EnglishHref:   localizedPath(localeEN, localPath),
		CurrentLocale: msg.LocaleName,
	}
}
