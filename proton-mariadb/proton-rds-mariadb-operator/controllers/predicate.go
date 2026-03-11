package controllers

import (
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

type ReconcilePredicate struct {
	predicate.Funcs
}

// Update implements default UpdateEvent filter for validating generation change.
func (ReconcilePredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		return false
	}
	if e.ObjectNew == nil {
		return false
	}

	return e.ObjectNew.GetGeneration() != e.ObjectOld.GetGeneration()
}
