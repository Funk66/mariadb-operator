/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	databasev1alpha1 "github.com/mmontes11/mariadb-operator/api/v1alpha1"
	"github.com/mmontes11/mariadb-operator/controllers/template"
	mariadbclient "github.com/mmontes11/mariadb-operator/pkg/mariadb"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	grantFinalizerName = "grant.database.mmontes.io/finalizer"
)

type wrappedGrantFinalizer struct {
	client.Client
	grant *databasev1alpha1.GrantMariaDB
}

func newWrappedGrantFinalizer(client client.Client, grant *databasev1alpha1.GrantMariaDB) template.WrappedFinalizer {
	return &wrappedGrantFinalizer{
		Client: client,
		grant:  grant,
	}
}

func (wf *wrappedGrantFinalizer) AddFinalizer(ctx context.Context) error {
	if wf.ContainsFinalizer() {
		return nil
	}
	return wf.patch(ctx, wf.grant, func(gmd *databasev1alpha1.GrantMariaDB) {
		controllerutil.AddFinalizer(wf.grant, grantFinalizerName)
	})
}

func (wf *wrappedGrantFinalizer) RemoveFinalizer(ctx context.Context) error {
	if !wf.ContainsFinalizer() {
		return nil
	}
	return wf.patch(ctx, wf.grant, func(gmd *databasev1alpha1.GrantMariaDB) {
		controllerutil.RemoveFinalizer(wf.grant, grantFinalizerName)
	})
}

func (wf *wrappedGrantFinalizer) ContainsFinalizer() bool {
	return controllerutil.ContainsFinalizer(wf.grant, grantFinalizerName)
}

func (wf *wrappedGrantFinalizer) Reconcile(ctx context.Context, mdbClient *mariadbclient.Client) error {
	err := wait.PollImmediateWithContext(ctx, 1*time.Second, 5*time.Second, func(ctx context.Context) (bool, error) {
		var user databasev1alpha1.UserMariaDB
		if err := wf.Get(ctx, userKey(wf.grant), &user); err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return true, err
		}
		return false, nil
	})
	// User does not exist
	if err == nil {
		return nil
	}
	if err != nil && !errors.Is(err, wait.ErrWaitTimeout) {
		return fmt.Errorf("error checking if user exists in MariaDB: %v", err)
	}

	opts := mariadbclient.GrantOpts{
		Privileges:  wf.grant.Spec.Privileges,
		Database:    wf.grant.Spec.Database,
		Table:       wf.grant.Spec.Table,
		Username:    wf.grant.Spec.Username,
		GrantOption: wf.grant.Spec.GrantOption,
	}
	if err := mdbClient.Revoke(ctx, opts); err != nil {
		return fmt.Errorf("error revoking grant in MariaDB: %v", err)
	}
	return nil
}

func (wf *wrappedGrantFinalizer) patch(ctx context.Context, grant *databasev1alpha1.GrantMariaDB,
	patchFn func(*databasev1alpha1.GrantMariaDB)) error {
	patch := client.MergeFrom(grant.DeepCopy())
	patchFn(grant)

	if err := wf.Client.Patch(ctx, grant, patch); err != nil {
		return fmt.Errorf("error patching GrantMariaDB: %v", err)
	}
	return nil
}
