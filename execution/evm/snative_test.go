// Copyright 2017 Monax Industries Limited
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package evm

import (
	"encoding/hex"
	"testing"

	"strings"

	acm "github.com/hyperledger/burrow/account"
	. "github.com/hyperledger/burrow/binary"
	"github.com/hyperledger/burrow/crypto"
	"github.com/hyperledger/burrow/execution/evm/abi"
	"github.com/hyperledger/burrow/execution/evm/asm/bc"
	"github.com/hyperledger/burrow/execution/evm/sha3"
	"github.com/hyperledger/burrow/permission"
	ptypes "github.com/hyperledger/burrow/permission/types"
	"github.com/stretchr/testify/assert"
)

// Compiling the Permissions solidity contract at
// (generated by with 'make snatives' function) and passing to
// https://ethereum.github.io/browser-solidity (toggle details to get list)
// yields:
// Keep this updated to drive TestPermissionsContractSignatures
const compiledSigs = `
a73f7f8a addRole(address,bytes32)
225b6574 hasBase(address,uint64)
ac4ab3fb hasRole(address,bytes32)
6853920e removeRole(address,bytes32)
dbd4a8ea setBase(address,uint64,bool)
c4bc7b70 setGlobal(uint64,bool)
b7d4dc0d unsetBase(address,uint64)
`

func TestPermissionsContractSignatures(t *testing.T) {
	contract := SNativeContracts()["Permissions"]

	nFuncs := len(contract.functions)

	sigMap := idToSignatureMap()

	assert.Len(t, sigMap, nFuncs,
		"Permissions contract defines %s functions so we need %s "+
			"signatures in compiledSigs",
		nFuncs, nFuncs)

	for funcID, signature := range sigMap {
		assertFunctionIDSignature(t, contract, funcID, signature)
	}
}

func TestSNativeContractDescription_Dispatch(t *testing.T) {
	contract := SNativeContracts()["Permissions"]
	state := newAppState()
	caller := acm.ConcreteAccount{
		Address: crypto.Address{1, 1, 1},
	}.MutableAccount()
	grantee := acm.ConcreteAccount{
		Address: crypto.Address{2, 2, 2},
	}.MutableAccount()
	state.UpdateAccount(grantee)

	function, err := contract.FunctionByName("addRole")
	if err != nil {
		t.Fatalf("Could not get function: %s", err)
	}
	funcID := function.ID()
	gas := uint64(1000)

	// Should fail since we have no permissions
	retValue, err := contract.Dispatch(state, caller, bc.MustSplice(funcID[:],
		grantee.Address(), permFlagToWord256(permission.CreateAccount)), &gas, logger)
	if !assert.Error(t, err, "Should fail due to lack of permissions") {
		return
	}
	assert.IsType(t, err, ErrLacksSNativePermission{})

	// Grant all permissions and dispatch should success
	caller.SetPermissions(allAccountPermissions())
	retValue, err = contract.Dispatch(state, caller, bc.MustSplice(funcID[:],
		grantee.Address().Word256(), permFlagToWord256(permission.CreateAccount)), &gas, logger)
	assert.NoError(t, err)
	assert.Equal(t, retValue, LeftPadBytes([]byte{1}, 32))
}

func TestSNativeContractDescription_Address(t *testing.T) {
	contract := NewSNativeContract("A comment",
		"CoolButVeryLongNamedContractOfDoom")
	assert.Equal(t, sha3.Sha3(([]byte)(contract.Name))[12:], contract.Address().Bytes())
}

//
// Helpers
//
func assertFunctionIDSignature(t *testing.T, contract *SNativeContractDescription,
	funcIDHex string, expectedSignature string) {
	fromHex := funcIDFromHex(t, funcIDHex)
	function, err := contract.FunctionByID(fromHex)
	assert.NoError(t, err,
		"Error retrieving SNativeFunctionDescription with ID %s", funcIDHex)
	if err == nil {
		assert.Equal(t, expectedSignature, function.Signature())
	}
}

func funcIDFromHex(t *testing.T, hexString string) abi.FunctionSelector {
	bs, err := hex.DecodeString(hexString)
	assert.NoError(t, err, "Could not decode hex string '%s'", hexString)
	if len(bs) != 4 {
		t.Fatalf("FunctionSelector must be 4 bytes but '%s' is %v bytes", hexString,
			len(bs))
	}
	return abi.FirstFourBytes(bs)
}

func permFlagToWord256(permFlag ptypes.PermFlag) Word256 {
	return Uint64ToWord256(uint64(permFlag))
}

func allAccountPermissions() ptypes.AccountPermissions {
	return ptypes.AccountPermissions{
		Base: ptypes.BasePermissions{
			Perms:  permission.AllPermFlags,
			SetBit: permission.AllPermFlags,
		},
		Roles: []string{},
	}
}

// turns the solidity compiler function summary into a map to drive signature
// test
func idToSignatureMap() map[string]string {
	sigMap := make(map[string]string)
	lines := strings.Split(compiledSigs, "\n")
	for _, line := range lines {
		trimmed := strings.Trim(line, " \t")
		if trimmed != "" {
			idSig := strings.Split(trimmed, " ")
			sigMap[idSig[0]] = idSig[1]
		}
	}
	return sigMap
}
