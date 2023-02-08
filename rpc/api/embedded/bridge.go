package embedded

import (
	"github.com/inconshreveable/log15"
	"github.com/pkg/errors"
	"github.com/zenon-network/go-zenon/chain/nom"
	"github.com/zenon-network/go-zenon/vm/constants"
	"github.com/zenon-network/go-zenon/vm/embedded/implementation"

	"github.com/zenon-network/go-zenon/chain"
	"github.com/zenon-network/go-zenon/common"
	"github.com/zenon-network/go-zenon/common/types"
	"github.com/zenon-network/go-zenon/rpc/api"
	"github.com/zenon-network/go-zenon/vm/embedded/definition"
	"github.com/zenon-network/go-zenon/vm/vm_context"
	"github.com/zenon-network/go-zenon/zenon"
)

type BridgeApi struct {
	chain chain.Chain
	log   log15.Logger
}

func NewBridgeApi(z zenon.Zenon) *BridgeApi {
	return &BridgeApi{
		chain: z.Chain(),
		log:   common.RPCLogger.New("module", "embedded_bridge_api"),
	}
}

func (a *BridgeApi) GetBridgeInfo() (*definition.BridgeInfoVariable, error) {
	_, context, err := api.GetFrontierContext(a.chain, types.BridgeContract)
	if err != nil {
		return nil, err
	}

	bridgeInfo, err := definition.GetBridgeInfoVariable(context.Storage())
	if err != nil {
		return nil, err
	}

	return bridgeInfo, nil
}

func (a *BridgeApi) GetSecurityInfo() (*definition.SecurityInfoVariable, error) {
	_, context, err := api.GetFrontierContext(a.chain, types.BridgeContract)
	if err != nil {
		return nil, err
	}

	security, err := definition.GetSecurityInfoVariable(context.Storage())
	if err != nil {
		return nil, err
	}

	return security, nil
}

func (a *BridgeApi) GetOrchestratorInfo() (*definition.OrchestratorInfo, error) {
	_, context, err := api.GetFrontierContext(a.chain, types.BridgeContract)
	if err != nil {
		return nil, err
	}

	orchestratorInfo, err := definition.GetOrchestratorInfoVariable(context.Storage())
	if err != nil {
		return nil, err
	}

	return orchestratorInfo, nil
}

func (a *BridgeApi) GetNetworkInfo(networkClass uint32, chainId uint32) (*definition.NetworkInfo, error) {
	_, context, err := api.GetFrontierContext(a.chain, types.BridgeContract)
	if err != nil {
		return nil, err
	}

	networkInfo, err := definition.GetNetworkInfoVariable(context.Storage(), networkClass, chainId)
	if err != nil {
		return nil, err
	}

	return networkInfo, nil
}

func (a *BridgeApi) GetAllNetworks(pageIndex, pageSize uint32) (*NetworkInfoList, error) {
	if pageSize > api.RpcMaxPageSize {
		return nil, api.ErrPageSizeParamTooBig
	}

	_, context, err := api.GetFrontierContext(a.chain, types.BridgeContract)
	if err != nil {
		return nil, err
	}
	networkList, err := definition.GetNetworkList(context.Storage())
	if err != nil {
		return nil, err
	}
	start, end := api.GetRange(pageIndex, pageSize, uint32(len(networkList)))
	list := networkList[start:end]

	result := &NetworkInfoList{
		Count: len(networkList),
		List:  list,
	}
	return result, nil
}

type NetworkInfoList struct {
	Count int                       `json:"count"`
	List  []*definition.NetworkInfo `json:"list"`
}

func (a *BridgeApi) toRequest(context vm_context.AccountVmContext, abiRequest *definition.WrapTokenRequest) *definition.WrapTokenRequest {
	if abiRequest == nil {
		return nil
	}
	networkInfoVariable, err := definition.GetNetworkInfoVariable(context.Storage(), abiRequest.NetworkClass, abiRequest.ChainId)
	if err != nil {
		return nil
	}
	tokenAddress := ""
	for i := 0; i < len(networkInfoVariable.TokenPairs); i++ {
		if networkInfoVariable.TokenPairs[i].TokenStandard == abiRequest.TokenStandard.String() {
			tokenAddress = networkInfoVariable.TokenPairs[i].TokenAddress
		}
	}
	if tokenAddress == "" {
		return nil
	}
	request := &definition.WrapTokenRequest{
		NetworkClass: abiRequest.NetworkClass,
		ChainId:      abiRequest.ChainId,
		Id:           abiRequest.Id,
		ToAddress:    abiRequest.ToAddress,
		TokenAddress: tokenAddress,
		Amount:       abiRequest.Amount,
		Signature:    abiRequest.Signature,
	}
	return request
}

type WrapTokenRequest struct {
	*definition.WrapTokenRequest
	TokenInfo               *api.Token `json:"token"`
	ConfirmationsToFinality uint64     `json:"confirmationsToFinality"`
}

func (a *BridgeApi) getToken(zts types.ZenonTokenStandard) (*api.Token, error) {
	_, context, err := api.GetFrontierContext(a.chain, types.TokenContract)
	if err != nil {
		return nil, err
	}
	tokenInfo, err := definition.GetTokenInfo(context.Storage(), zts)
	if err == constants.ErrDataNonExistent {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if tokenInfo != nil {
		return api.LedgerTokenInfoToRpc(tokenInfo), nil
	}
	return nil, nil
}

func (a *BridgeApi) getRedeemableIn(unwrapTokenRequest definition.UnwrapTokenRequest, tokenPair definition.TokenPair, momentum nom.Momentum) uint64 {
	var redeemableIn uint64
	if momentum.Height-unwrapTokenRequest.RegistrationMomentumHeight >= uint64(tokenPair.RedeemDelay) {
		redeemableIn = 0
	} else {
		redeemableIn = unwrapTokenRequest.RegistrationMomentumHeight + uint64(tokenPair.RedeemDelay) - momentum.Height
	}
	return redeemableIn
}

func (a *BridgeApi) getConfirmationsToFinality(wrapTokenRequest definition.WrapTokenRequest, confirmationsToFinality uint32, momentum nom.Momentum) (uint64, error) {
	var actualConfirmationsToFinality uint64
	if momentum.Height-wrapTokenRequest.CreationMomentumHeight >= uint64(confirmationsToFinality) {
		actualConfirmationsToFinality = 0
	} else {
		actualConfirmationsToFinality = wrapTokenRequest.CreationMomentumHeight + uint64(confirmationsToFinality) - momentum.Height
	}
	return actualConfirmationsToFinality, nil
}

func (a *BridgeApi) GetWrapTokenRequestById(id types.Hash) (*WrapTokenRequest, error) {
	_, context, err := api.GetFrontierContext(a.chain, types.BridgeContract)
	if err != nil {
		return nil, err
	}

	wrapTokenRequest, err := definition.GetWrapTokenRequestById(context.Storage(), id)
	if err != nil {
		return nil, err
	}

	token, err := a.getToken(wrapTokenRequest.TokenStandard)
	if err != nil {
		return nil, err
	}

	momentum, err := context.GetFrontierMomentum()
	if err != nil {
		return nil, err
	}
	orchestratorInfo, err := definition.GetOrchestratorInfoVariable(context.Storage())
	if err != nil {
		return nil, err
	}
	confirmationsToFinality, err := a.getConfirmationsToFinality(*wrapTokenRequest, orchestratorInfo.ConfirmationsToFinality, *momentum)
	if err != nil {
		return nil, err
	}

	return &WrapTokenRequest{wrapTokenRequest, token, confirmationsToFinality}, nil
}

type WrapTokenRequestList struct {
	Count int                 `json:"count"`
	List  []*WrapTokenRequest `json:"list"`
}

func (a *BridgeApi) GetAllWrapTokenRequests(pageIndex, pageSize uint32) (*WrapTokenRequestList, error) {
	_, context, err := api.GetFrontierContext(a.chain, types.BridgeContract)
	if err != nil {
		return nil, err
	}

	requests, err := definition.GetWrapTokenRequests(context.Storage())
	if err != nil {
		return nil, err
	}

	result := &WrapTokenRequestList{
		Count: len(requests),
		List:  make([]*WrapTokenRequest, 0),
	}

	momentum, err := context.GetFrontierMomentum()
	if err != nil {
		return nil, err
	}
	orchestratorInfo, err := definition.GetOrchestratorInfoVariable(context.Storage())
	if err != nil {
		return nil, err
	}

	start, end := api.GetRange(pageIndex, pageSize, uint32(len(requests)))
	for i := start; i < end; i++ {
		token, err := a.getToken(requests[i].TokenStandard)
		if err != nil {
			continue
		}
		confirmationsToFinality, err := a.getConfirmationsToFinality(*requests[i], orchestratorInfo.ConfirmationsToFinality, *momentum)
		if err != nil {
			continue
		}
		wrapReqest := &WrapTokenRequest{requests[i], token, confirmationsToFinality}
		result.List = append(result.List, wrapReqest)
	}
	return result, nil
}

func (a *BridgeApi) GetAllWrapTokenRequestsByToAddress(toAddress string, pageIndex, pageSize uint32) (*WrapTokenRequestList, error) {
	_, context, err := api.GetFrontierContext(a.chain, types.BridgeContract)
	if err != nil {
		return nil, err
	}

	requests, err := definition.GetWrapTokenRequests(context.Storage())
	if err != nil {
		return nil, err
	}

	result := &WrapTokenRequestList{
		List: make([]*WrapTokenRequest, 0),
	}

	specificRequests := make([]*definition.WrapTokenRequest, 0)
	if toAddress == "" {
		specificRequests = append(specificRequests, requests...)
	} else {
		for _, request := range requests {
			if request.ToAddress == toAddress {
				specificRequests = append(specificRequests, request)
			}
		}
	}
	result.Count = len(specificRequests)
	start, end := api.GetRange(pageIndex, pageSize, uint32(len(specificRequests)))

	momentum, err := context.GetFrontierMomentum()
	if err != nil {
		return nil, err
	}
	orchestratorInfo, err := definition.GetOrchestratorInfoVariable(context.Storage())
	if err != nil {
		return nil, err
	}
	for i := start; i < end; i++ {
		token, err := a.getToken(specificRequests[i].TokenStandard)
		if err != nil {
			continue
		}
		confirmationsToFinality, err := a.getConfirmationsToFinality(*specificRequests[i], orchestratorInfo.ConfirmationsToFinality, *momentum)
		if err != nil {
			continue
		}
		wrapRequest := &WrapTokenRequest{specificRequests[i], token, confirmationsToFinality}
		result.List = append(result.List, wrapRequest)
	}
	return result, nil
}

func (a *BridgeApi) GetAllWrapTokenRequestsByToAddressNetworkClassAndChainId(toAddress string, networkClass, chainId uint32, pageIndex, pageSize uint32) (*WrapTokenRequestList, error) {
	_, context, err := api.GetFrontierContext(a.chain, types.BridgeContract)
	if err != nil {
		return nil, err
	}

	requests, err := definition.GetWrapTokenRequests(context.Storage())
	if err != nil {
		return nil, err
	}

	result := &WrapTokenRequestList{
		List: make([]*WrapTokenRequest, 0),
	}

	specificRequests := make([]*definition.WrapTokenRequest, 0)
	for _, request := range requests {
		if request.NetworkClass == networkClass && request.ChainId == chainId && (toAddress == "" || request.ToAddress == toAddress) {
			specificRequests = append(specificRequests, request)
		}
	}
	result.Count = len(specificRequests)
	start, end := api.GetRange(pageIndex, pageSize, uint32(len(specificRequests)))

	momentum, err := context.GetFrontierMomentum()
	if err != nil {
		return nil, err
	}
	orchestratorInfo, err := definition.GetOrchestratorInfoVariable(context.Storage())
	if err != nil {
		return nil, err
	}

	for i := start; i < end; i++ {
		token, err := a.getToken(specificRequests[i].TokenStandard)
		if err != nil {
			continue
		}
		confirmationsToFinality, err := a.getConfirmationsToFinality(*specificRequests[i], orchestratorInfo.ConfirmationsToFinality, *momentum)
		if err != nil {
			continue
		}
		wrapRequest := &WrapTokenRequest{specificRequests[i], token, confirmationsToFinality}
		result.List = append(result.List, wrapRequest)
	}
	return result, nil
}

func (a *BridgeApi) GetAllUnsignedWrapTokenRequests(pageIndex, pageSize uint32) (*WrapTokenRequestList, error) {
	_, context, err := api.GetFrontierContext(a.chain, types.BridgeContract)
	if err != nil {
		return nil, err
	}

	requests, err := definition.GetWrapTokenRequests(context.Storage())
	if err != nil {
		return nil, err
	}
	var unsignedRequests []*WrapTokenRequest

	momentum, err := context.GetFrontierMomentum()
	if err != nil {
		return nil, err
	}
	orchestratorInfo, err := definition.GetOrchestratorInfoVariable(context.Storage())
	if err != nil {
		return nil, err
	}

	for _, request := range requests {
		if request.Signature == "" {
			token, err := a.getToken(request.TokenStandard)
			if err != nil {
				continue
			}
			confirmationsToFinality, err := a.getConfirmationsToFinality(*request, orchestratorInfo.ConfirmationsToFinality, *momentum)
			if err != nil {
				continue
			}
			wrapRequest := &WrapTokenRequest{request, token, confirmationsToFinality}
			unsignedRequests = append(unsignedRequests, wrapRequest)
		}
	}

	for i, j := 0, len(unsignedRequests)-1; i < j; i, j = i+1, j-1 {
		unsignedRequests[i], unsignedRequests[j] = unsignedRequests[j], unsignedRequests[i]
	}

	result := &WrapTokenRequestList{
		Count: len(unsignedRequests),
		List:  make([]*WrapTokenRequest, len(unsignedRequests)),
	}

	start, end := api.GetRange(pageIndex, pageSize, uint32(len(unsignedRequests)))
	result.List = unsignedRequests[start:end]
	return result, nil
}

type UnwrapTokenRequest struct {
	*definition.UnwrapTokenRequest
	TokenInfo    *api.Token `json:"token"`
	RedeemableIn uint64     `json:"redeemableIn"`
}

type UnwrapTokenRequestList struct {
	Count int                   `json:"count"`
	List  []*UnwrapTokenRequest `json:"list"`
}

func (a *BridgeApi) getTokenStandard(request *definition.UnwrapTokenRequest) (*types.ZenonTokenStandard, error) {
	networkInfo, err := a.GetNetworkInfo(request.NetworkClass, request.ChainId)
	if err != nil {
		return nil, err
	}
	tokenStandard := ""
	for _, pair := range networkInfo.TokenPairs {
		if pair.TokenAddress == request.TokenAddress {
			tokenStandard = pair.TokenStandard
			break
		}
	}
	if tokenStandard == "" {
		return nil, constants.ErrInvalidToken
	}
	zts := types.ParseZTSPanic(tokenStandard)
	return &zts, nil
}

func (a *BridgeApi) GetUnwrapTokenRequestByHashAndLog(txHash types.Hash, logIndex uint32) (*UnwrapTokenRequest, error) {
	_, context, err := api.GetFrontierContext(a.chain, types.BridgeContract)
	if err != nil {
		return nil, err
	}

	request, err := definition.GetUnwrapTokenRequestByTxHashAndLog(context.Storage(), txHash, logIndex)
	if err != nil {
		return nil, err
	}
	tokenStandard, err := a.getTokenStandard(request)
	if err != nil {
		return nil, err
	}
	token, err := a.getToken(*tokenStandard)
	if err != nil {
		return nil, err
	}
	momentum, err := context.GetFrontierMomentum()
	if err != nil {
		return nil, err
	}
	tokenPair, err := implementation.CheckNetworkAndPairExist(context, request.NetworkClass, request.ChainId, request.TokenAddress)
	if err != nil {
		return nil, err
	}
	if tokenPair == nil {
		return nil, errors.New("token pair not found")
	}

	redeemableIn := a.getRedeemableIn(*request, *tokenPair, *momentum)
	unwrapRequest := &UnwrapTokenRequest{request, token, redeemableIn}

	return unwrapRequest, nil
}

func (a *BridgeApi) GetAllUnwrapTokenRequests(pageIndex, pageSize uint32) (*UnwrapTokenRequestList, error) {
	_, context, err := api.GetFrontierContext(a.chain, types.BridgeContract)
	if err != nil {
		return nil, err
	}

	requests, err := definition.GetUnwrapTokenRequests(context.Storage())
	if err != nil {
		return nil, err
	}

	result := &UnwrapTokenRequestList{
		Count: len(requests),
		List:  make([]*UnwrapTokenRequest, 0),
	}

	start, end := api.GetRange(pageIndex, pageSize, uint32(len(requests)))
	momentum, err := context.GetFrontierMomentum()
	if err != nil {
		return nil, err
	}
	for i := start; i < end; i++ {
		zts, err := a.getTokenStandard(requests[i])
		if err != nil {
			continue
		}
		token, err := a.getToken(*zts)
		if err != nil {
			continue
		}
		tokenPair, err := implementation.CheckNetworkAndPairExist(context, requests[i].NetworkClass, requests[i].ChainId, requests[i].TokenAddress)
		if err != nil {
			return nil, err
		}
		if tokenPair == nil {
			return nil, errors.New("token pair not found")
		}
		redeemableIn := a.getRedeemableIn(*requests[i], *tokenPair, *momentum)
		result.List = append(result.List, &UnwrapTokenRequest{requests[i], token, redeemableIn})
	}
	return result, nil
}

func (a *BridgeApi) GetAllUnwrapTokenRequestsByToAddress(toAddress string, pageIndex, pageSize uint32) (*UnwrapTokenRequestList, error) {
	_, context, err := api.GetFrontierContext(a.chain, types.BridgeContract)
	if err != nil {
		return nil, err
	}

	requests, err := definition.GetUnwrapTokenRequests(context.Storage())
	if err != nil {
		return nil, err
	}

	result := &UnwrapTokenRequestList{
		List: make([]*UnwrapTokenRequest, 0),
	}
	specificRequests := make([]*definition.UnwrapTokenRequest, 0)
	if toAddress == "" {
		specificRequests = append(specificRequests, requests...)
	} else {
		for _, request := range requests {
			if request.ToAddress.String() == toAddress {
				specificRequests = append(specificRequests, request)
			}
		}

	}
	result.Count = len(specificRequests)
	start, end := api.GetRange(pageIndex, pageSize, uint32(len(specificRequests)))
	momentum, err := context.GetFrontierMomentum()
	if err != nil {
		return nil, err
	}
	for i := start; i < end; i++ {
		zts, err := a.getTokenStandard(specificRequests[i])
		if err != nil {
			continue
		}
		token, err := a.getToken(*zts)
		if err != nil {
			continue
		}
		tokenPair, err := implementation.CheckNetworkAndPairExist(context, specificRequests[i].NetworkClass, specificRequests[i].ChainId, specificRequests[i].TokenAddress)
		if err != nil {
			return nil, err
		}
		if tokenPair == nil {
			return nil, errors.New("token pair not found")
		}
		redeemableIn := a.getRedeemableIn(*specificRequests[i], *tokenPair, *momentum)
		result.List = append(result.List, &UnwrapTokenRequest{specificRequests[i], token, redeemableIn})
	}
	return result, nil
}

func (a *BridgeApi) GetFeeTokenPair(zts types.ZenonTokenStandard) (*definition.ZtsFeesInfo, error) {
	_, context, err := api.GetFrontierContext(a.chain, types.BridgeContract)
	if err != nil {
		return nil, err
	}
	return definition.GetZtsFeesInfoVariable(context.Storage(), zts.String())
}