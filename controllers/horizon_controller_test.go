package controllers

import (
	"context"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"

	horizonv1alpha1 "github.com/openstack-k8s-operators/horizon-operator/api/v1alpha1"
)

func TestFormatMemcachedServers(t *testing.T) {
	tests := []struct {
		serverList []string
		expected   string
	}{
		{[]string{"localhost"}, "'localhost:11211'"},
		{[]string{"server1", "server2", "server3"}, "'server1:11211','server2:11211','server3:11211'"},
		{[]string{}, ""},
		{[]string{"1.1.1.1", "2.2.2.2", "3.3.3.3"}, "'1.1.1.1:11211','2.2.2.2:11211','3.3.3.3:11211'"},
	}

	for _, tc := range tests {
		result := formatMemcachedServers(tc.serverList)

		if result != tc.expected {
			t.Errorf("formatMemcachedServers(%v) = %v, expected %v", tc.serverList, result, tc.expected)
		}
	}
}

func TestGetMemcachedServerList(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockOkoPod := NewMockOkoPodInterface(ctrl)

	expected := []string{"memcached1:11211", "memcached2:11211"}
	mockOkoPod.EXPECT().GetPodFQDNList(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(expected, nil)

	instance := &horizonv1alpha1.Horizon{}
	serverList, _ := getMemcachedServerList(context.Background(), nil, instance, "memcached-test", mockOkoPod)
	if !reflect.DeepEqual(serverList, expected) {
		t.Errorf("GetMemcachedServerList = %v, expected %v", serverList, expected)
	}
}
