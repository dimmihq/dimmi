/*
|--------------------------------------------------------------------------
| Refresh Metadata
|--------------------------------------------------------------------------
|
| A connection set up from its identity provider's metadata address keeps
| up with it: the provider rolls its signing certificate by publishing
| the new one there first. A refresh replaces the address, entity id and
| certificates with what is published; a failure keeps the old ones and
| tells the owners why. The address is fetched with the guard that keeps
| requests off private networks, and only a megabyte of it is read.
|
*/

package sso

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dimmihq/dimmi/framework/clock"
	"github.com/dimmihq/dimmi/framework/database"
	"github.com/dimmihq/dimmi/framework/httpclient"
	"github.com/dimmihq/dimmi/framework/saml"

	"github.com/dimmihq/dimmi/app/models"
)

/*
metadataMaxBytes and metadataTimeout bound one metadata fetch.
*/
const (
	metadataMaxBytes = 1 << 20
	metadataTimeout  = 15 * time.Second
)

/*
RefreshMetadata reads a SAML connection's identity provider settings
from its metadata address.
*/
type RefreshMetadata struct {
	DB    *database.DB
	HTTP  *httpclient.Client
	Clock clock.Clock
}

/*
Handle refreshes the connection, recording a failure on it rather than
returning it. Connections without a metadata address are left alone.
*/
func (a *RefreshMetadata) Handle(ctx context.Context, row models.SSOConnection) error {
	conn, err := Decode(row)
	if err != nil || row.Protocol != ProtocolSAML || conn.Config.IDPMetadataURL == "" {
		return err
	}

	now := a.Clock.Now()
	update := models.UpdateSSOConnectionMetadataParams{Config: row.Config, Now: now, ID: row.ID}

	idp, failure := a.fetch(ctx, conn.Config.IDPMetadataURL)
	if failure != "" {
		update.MetadataError = &failure
	} else {
		cfg := conn.Config
		cfg.IDPEntityID, cfg.IDPSSOURL, cfg.IDPCertificates = idp.EntityID, idp.SSOURL, idp.Certificates

		if update.Config, err = json.Marshal(cfg); err != nil {
			return fmt.Errorf("write refreshed config: %w", err)
		}

		update.RefreshedAt = &now
	}

	if err := models.New(a.DB).UpdateSSOConnectionMetadata(ctx, update); err != nil {
		return fmt.Errorf("store refreshed metadata: %w", err)
	}

	return nil
}

/*
published is what an identity provider's metadata says, its
certificates PEM encoded.
*/
type published struct {
	EntityID     string
	SSOURL       string
	Certificates []string
}

/*
fetch reads the metadata at the address, returning why it could not
instead of an error, since that reason is shown to the owners.
*/
func (a *RefreshMetadata) fetch(ctx context.Context, address string) (published, string) {
	if !strings.HasPrefix(address, "https://") && !strings.HasPrefix(address, "http://") {
		return published{}, "The metadata address must be a web address."
	}

	res, err := a.HTTP.New().Timeout(metadataTimeout).MaxBody(metadataMaxBytes).Get(ctx, address)
	if err != nil {
		return published{}, "The metadata address could not be reached."
	}

	if res.Status < 200 || res.Status > 299 {
		return published{}, fmt.Sprintf("The metadata address answered %d.", res.Status)
	}

	idp, err := saml.ParseMetadata(res.Body)
	if err != nil {
		return published{}, "The metadata does not describe a SAML identity provider with a redirect binding."
	}

	out := published{EntityID: idp.EntityID, SSOURL: idp.SSOURL}
	for _, certificate := range idp.Certificates {
		out.Certificates = append(out.Certificates, saml.EncodeCertificate(certificate))
	}

	return out, ""
}
