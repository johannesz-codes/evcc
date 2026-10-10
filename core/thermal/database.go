package thermal

import (
	"errors"

	"github.com/evcc-io/evcc/db/settings"
)

// DatabaseStore scopes ownership to a stable physical charger reference.
type DatabaseStore string

func (s DatabaseStore) Load(kind string, value any) error {
	err := settings.Json("thermal."+string(s)+"."+kind, value)
	if errors.Is(err, settings.ErrNotFound) {
		return nil
	}
	return err
}

func (s DatabaseStore) Save(kind string, value any) error {
	if err := settings.SetJson("thermal."+string(s)+"."+kind, value); err != nil {
		return err
	}
	return settings.Persist()
}
