package store

import (
	"strconv"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/yearn/ydaemon/common/addresses"
	"github.com/yearn/ydaemon/common/env"
	"github.com/yearn/ydaemon/common/logs"
	"github.com/yearn/ydaemon/internal/models"
	"gorm.io/gorm"
)

var _erc20SyncMap = make(map[uint64]*sync.Map)

/**************************************************************************************************
** LoadERC20 will retrieve the all the ERC20 tokens added to the configured DB and store them in
** the _erc20SyncMap for fast access during that
**************************************************************************************************/
func LoadERC20(chainID uint64, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}
	syncMap := _erc20SyncMap[chainID]
	if syncMap == nil {
		syncMap = &sync.Map{}
		_erc20SyncMap[chainID] = syncMap
	}

	switch _dbType {
	case DBBadger:
		logs.Warning(`BadgerDB is deprecated for LoadERC20`)
	case DBSql:
		var temp []DBERC20
		DATABASE.Table(`db_erc20`).
			Where("chain_id = ?", chainID).
			FindInBatches(&temp, 10_000, func(tx *gorm.DB, batch int) error {
				for _, tokenFromDB := range temp {
					token := models.TERC20Token{
						Address:       addresses.ToAddress(tokenFromDB.Address),
						Type:          tokenFromDB.Type,
						Name:          tokenFromDB.Name,
						Symbol:        tokenFromDB.Symbol,
						DisplayName:   tokenFromDB.DisplayName,
						DisplaySymbol: tokenFromDB.DisplaySymbol,
						Description:   tokenFromDB.Description,
						Icon:          env.BASE_ASSET_URL + strconv.FormatUint(chainID, 10) + `/` + tokenFromDB.Address + `/logo-128.png`,
						Decimals:      tokenFromDB.Decimals,
						ChainID:       tokenFromDB.ChainID,
					}
					allUnderlyingAsString := strings.Split(tokenFromDB.UnderlyingTokensAddresses, ",")
					for _, addressStr := range allUnderlyingAsString {
						token.UnderlyingTokensAddresses = append(token.UnderlyingTokensAddresses, common.HexToAddress(addressStr))
					}
					syncMap.Store(tokenFromDB.Address, token)
				}
				return nil
			})
	}
}

/**************************************************************************************************
** AppendToERC20 will add a new erc20 token in the _erc20SyncMap
**************************************************************************************************/
func AppendToERC20(chainID uint64, token models.TERC20Token) {
	syncMap := _erc20SyncMap[chainID]
	key := token.Address.Hex()
	syncMap.Store(key, token)
}

/**************************************************************************************************
** StoreERC20 will store a new erc20 token in the _erc20SyncMap for fast access during that same
** execution, and will store it in the configured DB for future executions.
**************************************************************************************************/
func StoreERC20(chainID uint64, token models.TERC20Token) {
	AppendToERC20(chainID, token)

	switch _dbType {
	case DBBadger:
		// LEGACY: Deprecated
		logs.Warning(`BadgerDB is deprecated for StoreERC20`)
	case DBSql:
		go func() {
			allUnderlyingAsString := []string{}
			for _, address := range token.UnderlyingTokensAddresses {
				allUnderlyingAsString = append(allUnderlyingAsString, address.Hex())
			}
			newItem := &DBERC20{
				UUID:                      getUUID(token.Address.Hex()),
				Address:                   token.Address.Hex(),
				Name:                      token.Name,
				Symbol:                    token.Symbol,
				Type:                      token.Type,
				DisplayName:               token.DisplayName,
				DisplaySymbol:             token.DisplaySymbol,
				Description:               token.Description,
				Decimals:                  token.Decimals,
				ChainID:                   token.ChainID,
				UnderlyingTokensAddresses: strings.Join(allUnderlyingAsString, ","),
			}
			wait(`StoreERC20`)
			DATABASE.
				Table(`db_erc20`).
				Where(`address = ? AND chain_id = ?`, newItem.Address, newItem.ChainID).
				Assign(newItem).
				FirstOrCreate(newItem)
		}()
	}
}

/**************************************************************************************************
** ListAllERC20 will return a list of all the ERC20 stored in the caching system for a given
** chainID. Both a map and a slice are returned.
**************************************************************************************************/
func ListAllERC20(chainID uint64) (asMap map[common.Address]models.TERC20Token, asSlice []models.TERC20Token) {
	asMap = make(map[common.Address]models.TERC20Token) // make to avoid nil map

	/**********************************************************************************************
	** We first retrieve the syncMap. This syncMap should be initialized first via the `LoadERC20`
	** function which take the data from the database/badger and store it in it.
	**********************************************************************************************/
	syncMap := _erc20SyncMap[chainID]
	if syncMap == nil {
		syncMap = &sync.Map{}
		_erc20SyncMap[chainID] = syncMap
	}

	/**********************************************************************************************
	** We can just iterate over the syncMap and add the vaults to the map and slice.
	** As the stored vault data are only a subset of static, we need to use the actual structure
	** and not the DBVault one.
	**********************************************************************************************/
	syncMap.Range(func(key, value interface{}) bool {
		token := value.(models.TERC20Token)
		asMap[token.Address] = token
		asSlice = append(asSlice, token)
		return true
	})

	return asMap, asSlice
}

/**************************************************************************************************
** GetERC20 will try, for a specific chain, to find the provided token address as ERC20 struct.
**************************************************************************************************/
func GetERC20(chainID uint64, tokenAddress common.Address) (token models.TERC20Token, ok bool) {
	tokenFromSyncMap, ok := _erc20SyncMap[chainID].Load(tokenAddress.Hex())
	if !ok {
		return models.TERC20Token{}, false
	}
	return tokenFromSyncMap.(models.TERC20Token), true
}
