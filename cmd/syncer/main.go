package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/grafana-tools/sdk"
	"github.com/tidwall/sjson"

	"github.com/mintel/grafana-local-sync/cmd/syncer/dashboard"
)

var (
	grafanaAddr = flag.String("addr", "http://localhost:3000", "The address Grafana is listening on.")
	dir         = flag.String("dir", ".", "The directory of Grafana dashboards to be synced.")

	username = flag.String("user", "", "")
	password = flag.String("pass", "", "")

	apiKey = flag.String("key", "", "")
)

func main() {
	flag.Parse()

	var authStr string
	if apiKey != nil && *apiKey != "" {
		authStr = *apiKey
	} else if username != nil && password != nil && *username != "" && *password != "" {
		authStr = *username + ":" + *password
	} else {
		log.Fatal("either -user/-pass, or -key, must be passed")
	}

	client, err := sdk.NewClient(*grafanaAddr, authStr, &http.Client{})
	if err != nil {
		log.Fatalf("error creating Grafana client: %s", err)
	}

	if err := syncLocalToRemote(context.Background(), client, *dir); err != nil {
		log.Fatal(err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)
	firstC := make(chan time.Time, 1)
	firstC <- time.Now()
	var c <-chan time.Time = firstC

	for {
		select {
		case <-done:
			return
		case <-c:
			log.Print("Syncing Grafana dashboards to local directory...")
			if err := syncRemoteToLocal(context.Background(), client, *dir); err != nil {
				log.Fatal(err)
			}
			c = time.After(30 * time.Second)
			log.Print("done.")
		}
	}
}

func syncRemoteToLocal(ctx context.Context, c *sdk.Client, dir string) error {
	localDashboards, err := listLocalDashboards(dir)
	if err != nil {
		return err
	}

	if err = ValidateDashboards(localDashboards); err != nil {
		return err
	}

	remoteDashboards, err := listRemoteDashboards(ctx, c)
	if err != nil {
		return err
	}

	remoteDashboards.Each(func(db dashboard.Dashboard) bool {
		var b []byte
		b, _, err = c.GetRawDashboardByUID(ctx, db.UID)
		if err != nil {
			return false
		}

		// Remove the "id" key; this ID is specific to the Grafana instance.
		b, err = sjson.DeleteBytes(b, "id")
		if err != nil {
			return false
		}

		// Use pretty indentation for nicer MRs.
		buf := bytes.Buffer{}
		if err = json.Indent(&buf, b, "", "  "); err != nil {
			return false
		}

		log.Printf("Writing dashboard '%s / %s' to %s", db.FolderTitle, db.Title, filepath.Join(dir, db.Filename))
		if err = ioutil.WriteFile(filepath.Join(dir, db.Filename), buf.Bytes(), 0644); err != nil {
			return false
		}
		return true
	})
	if err != nil {
		return err
	}

	toDelete := dashboard.Difference(localDashboards, remoteDashboards)
	toDelete.Each(func(db dashboard.Dashboard) bool {
		err = os.Remove(filepath.Join(dir, db.Filename))
		return err == nil
	})

	return err
}

func syncLocalToRemote(ctx context.Context, c *sdk.Client, dir string) error {
	localDashboards, err := listLocalDashboards(dir)
	if err != nil {
		return err
	}

	if err = ValidateDashboards(localDashboards); err != nil {
		return err
	}

	localDashboards.Each(func(db dashboard.Dashboard) bool {
		var b []byte
		b, err = os.ReadFile(filepath.Join(dir, db.Filename))
		if err != nil {
			return false
		}

		var possibleFolders []sdk.FoundBoard
		possibleFolders, err = c.Search(ctx,
			sdk.SearchType(sdk.SearchTypeFolder),
			sdk.SearchQuery(db.FolderTitle),
		)
		if err != nil {
			return false
		}
		f := sdk.Folder{
			ID: -1,
		}
		for _, fb := range possibleFolders {
			if fb.Title == db.FolderTitle {
				f.ID = int(fb.ID)
				break
			}
		}
		if f.ID == -1 {
			log.Printf("Creating folder %s", db.FolderTitle)
			f, err = c.CreateFolder(ctx, sdk.Folder{Title: db.FolderTitle})
			if err != nil && f.Version < 1 {
				// HTTP 409 == Folder already exists, which is fine.
				// The SDK doesn't provide an easy way to test HTTP status codes,
				// but I'm going to assume if we got back a folder version, it exists.
				return false
			}
		}

		log.Printf("Sending %s to Grafana instance", db.Filename)
		_, err = c.SetRawDashboardWithParam(ctx, sdk.RawBoardRequest{
			Dashboard: b,
			Parameters: sdk.SetDashboardParams{
				FolderID:  f.ID,
				Overwrite: true,
			},
		})
		return err == nil
	})
	return err
}

func listLocalDashboards(dir string) (*dashboard.Set, error) {
	dashboards := dashboard.NewSet()
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name()[0] == '.' || filepath.Ext(path) != ".json" {
			return nil // Skip hidden or non-json stuff.
		}
		db, err := dashboard.NewFromFile(dir, path)
		if err != nil {
			return err
		}
		dashboards.Add(db)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return dashboards, nil
}

func listRemoteDashboards(ctx context.Context, c *sdk.Client) (*dashboard.Set, error) {
	dashboards, err := c.Search(
		context.Background(),
		sdk.SearchType(sdk.SearchTypeDashboard),
	)
	if err != nil {
		return nil, err
	}
	result := dashboard.NewSetWithSize(len(dashboards))
	for _, db := range dashboards {
		result.Add(dashboard.NewFromFoundBoard(db))
	}
	return result, nil
}

func ValidateDashboards(dbs *dashboard.Set) (err error) {
	folderTitles := make(map[string]struct{})
	dbs.Each(func(item dashboard.Dashboard) bool {
		if item.Title == item.FolderTitle {
			err = fmt.Errorf("error: cannot have a dashboard with the same name as the folder it is in (%s)", item.Filename)
			return false
		}
		folderTitles[item.FolderTitle] = struct{}{}
		return true
	})
	if err != nil {
		return err
	}
	dbs.Each(func(item dashboard.Dashboard) bool {
		if _, ok := folderTitles[item.Title]; ok {
			err = fmt.Errorf("error: cannot have a dashboard with the same name as a folder (%s)", item.Title)
			return false
		}
		return true
	})
	return err
}
