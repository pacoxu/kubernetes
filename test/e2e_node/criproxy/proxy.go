package criproxy

type CriProxy interface {
	SetErrorInjectors(errorInjectors func(string) error)
}
