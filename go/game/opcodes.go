// Code generated from AL-Game's ServerPacketsOpcodes and AionPacketHandlerFactory; DO NOT EDIT.

package game

// Server packet opcodes, before crypt.EncodeServerOpcode.
const (
	smStatupdateMp              = 0x00    // SM_STATUPDATE_MP
	smStatupdateHp              = 0x01    // SM_STATUPDATE_HP
	smChatInit                  = 0x02    // SM_CHAT_INIT
	smChannelInfo               = 0x03    // SM_CHANNEL_INFO
	smMacroResult               = 0x04    // SM_MACRO_RESULT
	smMacroList                 = 0x05    // SM_MACRO_LIST
	smNicknameCheckResponse     = 0x07    // SM_NICKNAME_CHECK_RESPONSE
	smRiftAnnounce              = 0x08    // SM_RIFT_ANNOUNCE
	smSetBindPoint              = 0x09    // SM_SET_BIND_POINT
	smAbyssRank                 = 0x0b    // SM_ABYSS_RANK
	smFriendUpdate              = 0x0c    // SM_FRIEND_UPDATE
	smPetition                  = 0x0d    // SM_PETITION
	smRecipeDelete              = 0x0e    // SM_RECIPE_DELETE
	smLearnRecipe               = 0x0f    // SM_LEARN_RECIPE
	smFlyTime                   = 0x10    // SM_FLY_TIME
	smDelete                    = 0x12    // SM_DELETE
	smPlayerMove                = 0x13    // SM_PLAYER_MOVE
	smMessage                   = 0x14    // SM_MESSAGE
	smLoginQueue                = 0x15    // SM_LOGIN_QUEUE
	smInventoryInfo             = 0x16    // SM_INVENTORY_INFO
	smSystemMessage             = 0x17    // SM_SYSTEM_MESSAGE
	smDeleteItem                = 0x18    // SM_DELETE_ITEM
	smAddItems                  = 0x19    // SM_ADD_ITEMS
	smUiSettings                = 0x1a    // SM_UI_SETTINGS
	smUpdateItem                = 0x1b    // SM_UPDATE_ITEM
	smPlayerInfo                = 0x1c    // SM_PLAYER_INFO
	smGatherStatus              = 0x1e    // SM_GATHER_STATUS
	smCastspell                 = 0x1f    // SM_CASTSPELL
	smUpdatePlayerAppearance    = 0x20    // SM_UPDATE_PLAYER_APPEARANCE
	smGatherUpdate              = 0x21    // SM_GATHER_UPDATE
	smStatupdateDp              = 0x22    // SM_STATUPDATE_DP
	smAttackStatus              = 0x23    // SM_ATTACK_STATUS
	smStatupdateExp             = 0x24    // SM_STATUPDATE_EXP
	smDpInfo                    = 0x25    // SM_DP_INFO
	smLegionTabs                = 0x28    // SM_LEGION_TABS
	smLegionUpdateNickname      = 0x29    // SM_LEGION_UPDATE_NICKNAME
	smNpcInfo                   = 0x2a    // SM_NPC_INFO
	smEnterWorldCheck           = 0x2b    // SM_ENTER_WORLD_CHECK
	smPlayerSpawn               = 0x2d    // SM_PLAYER_SPAWN
	smGatherableInfo            = 0x2f    // SM_GATHERABLE_INFO
	smTeleportLoc               = 0x30    // SM_TELEPORT_LOC
	smAttack                    = 0x32    // SM_ATTACK
	smMove                      = 0x35    // SM_MOVE
	smTransform                 = 0x36    // SM_TRANSFORM
	smDialogWindow              = 0x38    // SM_DIALOG_WINDOW
	smSellItem                  = 0x3a    // SM_SELL_ITEM
	smViewPlayerDetails         = 0x3f    // SM_VIEW_PLAYER_DETAILS
	smPlayerState               = 0x40    // SM_PLAYER_STATE
	smWeather                   = 0x41    // SM_WEATHER
	smGameTime                  = 0x42    // SM_GAME_TIME
	smEmotion                   = 0x43    // SM_EMOTION
	smLookatobject              = 0x44    // SM_LOOKATOBJECT
	smTimeCheck                 = 0x45    // SM_TIME_CHECK
	smSkillCancel               = 0x46    // SM_SKILL_CANCEL
	smTargetSelected            = 0x47    // SM_TARGET_SELECTED
	smSkillList                 = 0x48    // SM_SKILL_LIST
	smCastspellEnd              = 0x49    // SM_CASTSPELL_END
	smSkillActivation           = 0x4a    // SM_SKILL_ACTIVATION
	smSkillRemove               = 0x4b    // SM_SKILL_REMOVE
	smAbnormalEffect            = 0x4e    // SM_ABNORMAL_EFFECT
	smAbnormalState             = 0x4f    // SM_ABNORMAL_STATE
	smQuestionWindow            = 0x50    // SM_QUESTION_WINDOW
	smSkillCooldown             = 0x51    // SM_SKILL_COOLDOWN
	smInfluenceRatio            = 0x53    // SM_INFLUENCE_RATIO
	smNameChange                = 0x54    // SM_NAME_CHANGE
	smGroupInfo                 = 0x56    // SM_GROUP_INFO
	smShowNpcOnMap              = 0x57    // SM_SHOW_NPC_ON_MAP
	smGroupMemberInfo           = 0x59    // SM_GROUP_MEMBER_INFO
	smQuitResponse              = 0x5e    // SM_QUIT_RESPONSE
	smLevelUpdate               = 0x62    // SM_LEVEL_UPDATE
	smKey                       = 0x64    // SM_KEY
	smExchangeRequest           = 0x66    // SM_EXCHANGE_REQUEST
	smSummonPanelRemove         = 0x67    // SM_SUMMON_PANEL_REMOVE
	smExchangeAddItem           = 0x69    // SM_EXCHANGE_ADD_ITEM
	smExchangeConfirmation      = 0x6a    // SM_EXCHANGE_CONFIRMATION
	smExchangeAddKinah          = 0x6b    // SM_EXCHANGE_ADD_KINAH
	smEmotionList               = 0x6d    // SM_EMOTION_LIST
	smTargetUpdate              = 0x6f    // SM_TARGET_UPDATE
	smLegionUpdateSelfIntro     = 0x75    // SM_LEGION_UPDATE_SELF_INTRO
	smRiftStatus                = 0x76    // SM_RIFT_STATUS
	smQuestAccepted             = 0x78    // SM_QUEST_ACCEPTED
	smQuestList                 = 0x79    // SM_QUEST_LIST
	smPingResponse              = 0x7c    // SM_PING_RESPONSE
	smNearbyQuests              = 0x7d    // SM_NEARBY_QUESTS
	smCubeUpdate                = 0x7e    // SM_CUBE_UPDATE
	smFriendList                = 0x80    // SM_FRIEND_LIST
	smUpdateNote                = 0x84    // SM_UPDATE_NOTE
	smItemCooldown              = 0x85    // SM_ITEM_COOLDOWN
	smPlayMovie                 = 0x87    // SM_PLAY_MOVIE
	smLegionInfo                = 0x8a    // SM_LEGION_INFO
	smLegionLeaveMember         = 0x8c    // SM_LEGION_LEAVE_MEMBER
	smLegionAddMember           = 0x8d    // SM_LEGION_ADD_MEMBER
	smLegionUpdateTitle         = 0x8e    // SM_LEGION_UPDATE_TITLE
	smLegionUpdateMember        = 0x8f    // SM_LEGION_UPDATE_MEMBER
	smBrokerRegistrationService = 0x93    // SM_BROKER_REGISTRATION_SERVICE
	smBrokerSettledList         = 0x95    // SM_BROKER_SETTLED_LIST
	smSummonOwnerRemove         = 0x96    // SM_SUMMON_OWNER_REMOVE
	smSummonPanel               = 0x97    // SM_SUMMON_PANEL
	smSummonUpdate              = 0x99    // SM_SUMMON_UPDATE
	smLegionEdit                = 0x9a    // SM_LEGION_EDIT
	smLegionMemberlist          = 0x9b    // SM_LEGION_MEMBERLIST
	smSummonUseskill            = 0x9e    // SM_SUMMON_USESKILL
	smMailService               = 0x9f    // SM_MAIL_SERVICE
	smPrivateStore              = 0xa2    // SM_PRIVATE_STORE
	smAbyssRankUpdate           = 0xa4    // SM_ABYSS_RANK_UPDATE
	smGroupLoot                 = 0xa5    // SM_GROUP_LOOT
	smMayLoginIntoGame          = 0xa7    // SM_MAY_LOGIN_INTO_GAME
	smPong                      = 0xaa    // SM_PONG
	smPlayerId                  = 0xab    // SM_PLAYER_ID
	smKiskUpdate                = 0xac    // SM_KISK_UPDATE
	smBrokerItems               = 0xae    // SM_BROKER_ITEMS
	smPrivateStoreName          = 0xaf    // SM_PRIVATE_STORE_NAME
	smBrokerRegisteredList      = 0xb1    // SM_BROKER_REGISTERED_LIST
	smAscensionMorph            = 0xb2    // SM_ASCENSION_MORPH
	smCraftUpdate               = 0xb3    // SM_CRAFT_UPDATE
	smCustomSettings            = 0xb4    // SM_CUSTOM_SETTINGS
	smItemUsageAnimation        = 0xb5    // SM_ITEM_USAGE_ANIMATION
	smDuel                      = 0xb7    // SM_DUEL
	smResurrect                 = 0xbe    // SM_RESURRECT
	smDie                       = 0xbf    // SM_DIE
	smTeleportMap               = 0xc0    // SM_TELEPORT_MAP
	smForcedMove                = 0xc1    // SM_FORCED_MOVE
	smWarehouseInfo             = 0xc4    // SM_WAREHOUSE_INFO
	smDeleteWarehouseItem       = 0xc6    // SM_DELETE_WAREHOUSE_ITEM
	smWarehouseUpdate           = 0xc7    // SM_WAREHOUSE_UPDATE
	smUpdateWarehouseItem       = 0xc9    // SM_UPDATE_WAREHOUSE_ITEM
	smTitleList                 = 0xcc    // SM_TITLE_LIST
	smTitleSet                  = 0xcf    // SM_TITLE_SET
	smCraftAnimation            = 0xd0    // SM_CRAFT_ANIMATION
	smTitleUpdate               = 0xd1    // SM_TITLE_UPDATE
	smLegionSendEmblem          = 0xd3    // SM_LEGION_SEND_EMBLEM
	smLegionUpdateEmblem        = 0xd5    // SM_LEGION_UPDATE_EMBLEM
	smFriendResponse            = 0xda    // SM_FRIEND_RESPONSE
	smBlockList                 = 0xdc    // SM_BLOCK_LIST
	smBlockResponse             = 0xdd    // SM_BLOCK_RESPONSE
	smFriendNotify              = 0xdf    // SM_FRIEND_NOTIFY
	smUseObject                 = 0xe3    // SM_USE_OBJECT
	smCharacterList             = 0xe4    // SM_CHARACTER_LIST
	smL2authLoginCheck          = 0xe5    // SM_L2AUTH_LOGIN_CHECK
	smDeleteCharacter           = 0xe6    // SM_DELETE_CHARACTER
	smCreateCharacter           = 0xe7    // SM_CREATE_CHARACTER
	smTargetImmobilize          = 0xe8    // SM_TARGET_IMMOBILIZE
	smRestoreCharacter          = 0xe9    // SM_RESTORE_CHARACTER
	smLootItemlist              = 0xea    // SM_LOOT_ITEMLIST
	smLootStatus                = 0xeb    // SM_LOOT_STATUS
	smMantraEffect              = 0xec    // SM_MANTRA_EFFECT
	smRecipeList                = 0xed    // SM_RECIPE_LIST
	smSiegeLocationInfo         = 0xef    // SM_SIEGE_LOCATION_INFO
	smPlayerSearch              = 0xf1    // SM_PLAYER_SEARCH
	smAllianceMemberInfo        = 0xf2    // SM_ALLIANCE_MEMBER_INFO
	smAllianceInfo              = 0xf3    // SM_ALLIANCE_INFO
	smLeaveGroupMember          = 0xf5    // SM_LEAVE_GROUP_MEMBER
	smAllianceReadyCheck        = 0xf6    // SM_ALLIANCE_READY_CHECK
	smShowBrand                 = 0xf7    // SM_SHOW_BRAND
	smPrices                    = 0xf8    // SM_PRICES
	smTradelist                 = 0xfb    // SM_TRADELIST
	smVersionCheck              = 0xfc    // SM_VERSION_CHECK
	smReconnectKey              = 0xfd    // SM_RECONNECT_KEY
	smStatsInfo                 = 0xff    // SM_STATS_INFO
	smQuestionnaire             = 0xbd    // SM_QUESTIONNAIRE
	smCustomPacket              = 0x1869f // SM_CUSTOM_PACKET
)

// Client packet opcodes.
const (
	cmCraft                  = 0x00 // CM_CRAFT
	cmClientCommandLoc       = 0x01 // CM_CLIENT_COMMAND_LOC
	cmRestoreCharacter       = 0x04 // CM_RESTORE_CHARACTER
	cmStartLoot              = 0x05 // CM_START_LOOT
	cmLootItem               = 0x06 // CM_LOOT_ITEM
	cmMoveItem               = 0x07 // CM_MOVE_ITEM
	cmL2authLoginCheck       = 0x08 // CM_L2AUTH_LOGIN_CHECK
	cmCharacterList          = 0x09 // CM_CHARACTER_LIST
	cmCreateCharacter        = 0x0a // CM_CREATE_CHARACTER
	cmDeleteCharacter        = 0x0b // CM_DELETE_CHARACTER
	cmLegionUploadEmblem     = 0x0c // CM_LEGION_UPLOAD_EMBLEM
	cmSplitItem              = 0x10 // CM_SPLIT_ITEM
	cmPlayerSearch           = 0x12 // CM_PLAYER_SEARCH
	cmLegionUploadInfo       = 0x13 // CM_LEGION_UPLOAD_INFO
	cmFriendStatus           = 0x15 // CM_FRIEND_STATUS
	cmChangeChannel          = 0x17 // CM_CHANGE_CHANNEL
	cmBlockAdd               = 0x19 // CM_BLOCK_ADD
	cmBlockDel               = 0x1a // CM_BLOCK_DEL
	cmShowBlocklist          = 0x1b // CM_SHOW_BLOCKLIST
	cmCheckNickname          = 0x1c // CM_CHECK_NICKNAME
	cmReplaceItem            = 0x1d // CM_REPLACE_ITEM
	cmBlockSetReason         = 0x1e // CM_BLOCK_SET_REASON
	cmMacAddress2            = 0x21 // CM_MAC_ADDRESS2
	cmMacroCreate            = 0x22 // CM_MACRO_CREATE
	cmMacroDelete            = 0x23 // CM_MACRO_DELETE
	cmDistributionSettings   = 0x24 // CM_DISTRIBUTION_SETTINGS
	cmMayLoginIntoGame       = 0x25 // CM_MAY_LOGIN_INTO_GAME
	cmShowBrand              = 0x28 // CM_SHOW_BRAND
	cmReconnectAuth          = 0x2a // CM_RECONNECT_AUTH
	cmGroupLoot              = 0x2b // CM_GROUP_LOOT
	cmShowMap                = 0x2f // CM_SHOW_MAP
	cmMacAddress             = 0x30 // CM_MAC_ADDRESS
	cmReportPlayer           = 0x32 // CM_REPORT_PLAYER
	cmGroupResponse          = 0x33 // CM_GROUP_RESPONSE
	cmSummonMove             = 0x34 // CM_SUMMON_MOVE
	cmSummonEmotion          = 0x35 // CM_SUMMON_EMOTION
	cmSummonAttack           = 0x36 // CM_SUMMON_ATTACK
	cmDeleteQuest            = 0x43 // CM_DELETE_QUEST
	cmItemRemodel            = 0x45 // CM_ITEM_REMODEL
	cmGodstoneSocket         = 0x46 // CM_GODSTONE_SOCKET
	cmInviteToGroup          = 0x4c // CM_INVITE_TO_GROUP
	cmAllianceGroupChange    = 0x4d // CM_ALLIANCE_GROUP_CHANGE
	cmViewPlayerDetails      = 0x4f // CM_VIEW_PLAYER_DETAILS
	cmPlayerStatusInfo       = 0x53 // CM_PLAYER_STATUS_INFO
	cmClientCommandRoll      = 0x56 // CM_CLIENT_COMMAND_ROLL
	cmGroupDistribution      = 0x57 // CM_GROUP_DISTRIBUTION
	cmPingRequest            = 0x5a // CM_PING_REQUEST
	cmDuelRequest            = 0x5d // CM_DUEL_REQUEST
	cmDeleteItem             = 0x5f // CM_DELETE_ITEM
	cmShowFriendlist         = 0x61 // CM_SHOW_FRIENDLIST
	cmFriendAdd              = 0x62 // CM_FRIEND_ADD
	cmFriendDel              = 0x63 // CM_FRIEND_DEL
	cmSummonCommand          = 0x64 // CM_SUMMON_COMMAND
	cmBrokerList             = 0x66 // CM_BROKER_LIST
	cmPrivateStore           = 0x6a // CM_PRIVATE_STORE
	cmPrivateStoreName       = 0x6b // CM_PRIVATE_STORE_NAME
	cmBrokerSettleList       = 0x6c // CM_BROKER_SETTLE_LIST
	cmBrokerSettleAccount    = 0x6d // CM_BROKER_SETTLE_ACCOUNT
	cmSendMail               = 0x6f // CM_SEND_MAIL
	cmBrokerRegistered       = 0x70 // CM_BROKER_REGISTERED
	cmBuyBrokerItem          = 0x71 // CM_BUY_BROKER_ITEM
	cmRegisterBrokerItem     = 0x72 // CM_REGISTER_BROKER_ITEM
	cmBrokerCancelRegistered = 0x73 // CM_BROKER_CANCEL_REGISTERED
	cmDeleteMail             = 0x74 // CM_DELETE_MAIL
	cmTitleSet               = 0x76 // CM_TITLE_SET
	cmReadMail               = 0x79 // CM_READ_MAIL
	cmGetMailAttachment      = 0x7b // CM_GET_MAIL_ATTACHMENT
	cmTeleportSelect         = 0x7f // CM_TELEPORT_SELECT
	cmPetition               = 0x85 // CM_PETITION
	cmChatMessagePublic      = 0x86 // CM_CHAT_MESSAGE_PUBLIC
	cmChatMessageWhisper     = 0x87 // CM_CHAT_MESSAGE_WHISPER
	cmOpenStaticdoor         = 0x8a // CM_OPEN_STATICDOOR
	cmCastspell              = 0x8c // CM_CASTSPELL
	cmSkillDeactivate        = 0x8d // CM_SKILL_DEACTIVATE
	cmRemoveAlteredState     = 0x8e // CM_REMOVE_ALTERED_STATE
	cmTargetSelect           = 0x92 // CM_TARGET_SELECT
	cmAttack                 = 0x93 // CM_ATTACK
	cmEmotion                = 0x96 // CM_EMOTION
	cmPing                   = 0x97 // CM_PING
	cmUseItem                = 0x98 // CM_USE_ITEM
	cmEquipItem              = 0x99 // CM_EQUIP_ITEM
	cmFlightTeleport         = 0x9c // CM_FLIGHT_TELEPORT
	cmQuestionResponse       = 0x9d // CM_QUESTION_RESPONSE
	cmBuyItem                = 0x9e // CM_BUY_ITEM
	cmShowDialog             = 0x9f // CM_SHOW_DIALOG
	cmLegion                 = 0xa0 // CM_LEGION
	cmMove                   = 0xa3 // CM_MOVE
	cmSetNote                = 0xa5 // CM_SET_NOTE
	cmLegionModifyEmblem     = 0xa6 // CM_LEGION_MODIFY_EMBLEM
	cmCloseDialog            = 0xa8 // CM_CLOSE_DIALOG
	cmDialogSelect           = 0xa9 // CM_DIALOG_SELECT
	cmLegionTabs             = 0xaa // CM_LEGION_TABS
	cmExchangeAddKinah       = 0xad // CM_EXCHANGE_ADD_KINAH
	cmExchangeLock           = 0xae // CM_EXCHANGE_LOCK
	cmExchangeOk             = 0xaf // CM_EXCHANGE_OK
	cmExchangeRequest        = 0xb2 // CM_EXCHANGE_REQUEST
	cmExchangeAddItem        = 0xb3 // CM_EXCHANGE_ADD_ITEM
	cmManastone              = 0xb5 // CM_MANASTONE
	cmExchangeCancel         = 0xb8 // CM_EXCHANGE_CANCEL
	cmPlayMovieEnd           = 0xbc // CM_PLAY_MOVIE_END
	cmSummonCastspell        = 0xc0 // CM_SUMMON_CASTSPELL
	cmFusionWeapons          = 0xc1 // CM_FUSION_WEAPONS
	cmBreakWeapons           = 0xc2 // CM_BREAK_WEAPONS
	cmLegionSendEmblem       = 0xd3 // CM_LEGION_SEND_EMBLEM
	cmDisconnect             = 0xed // CM_DISCONNECT
	cmQuit                   = 0xee // CM_QUIT
	cmMayQuit                = 0xef // CM_MAY_QUIT
	cmVersionCheck           = 0xf3 // CM_VERSION_CHECK
	cmLevelReady             = 0xf4 // CM_LEVEL_READY
	cmUiSettings             = 0xf5 // CM_UI_SETTINGS
	cmObjectSearch           = 0xf6 // CM_OBJECT_SEARCH
	cmCustomSettings         = 0xf7 // CM_CUSTOM_SETTINGS
	cmRevive                 = 0xf8 // CM_REVIVE
	cmEnterWorld             = 0xfb // CM_ENTER_WORLD
	cmTimeCheck              = 0xfd // CM_TIME_CHECK
	cmGather                 = 0xfe // CM_GATHER
	cmQuestionnaire          = 0x7c // CM_QUESTIONNAIRE
)

// clientPacketStates is the connection states each client packet is accepted in.
var clientPacketStates = map[byte]stateSet{
	cmCraft:                  inGame,
	cmClientCommandLoc:       inGame,
	cmRestoreCharacter:       inAuthed,
	cmStartLoot:              inGame,
	cmLootItem:               inGame,
	cmMoveItem:               inGame,
	cmL2authLoginCheck:       inConnected,
	cmCharacterList:          inAuthed,
	cmCreateCharacter:        inAuthed,
	cmDeleteCharacter:        inAuthed,
	cmLegionUploadEmblem:     inGame,
	cmSplitItem:              inGame,
	cmPlayerSearch:           inGame,
	cmLegionUploadInfo:       inGame,
	cmFriendStatus:           inGame,
	cmChangeChannel:          inGame,
	cmBlockAdd:               inGame,
	cmBlockDel:               inGame,
	cmShowBlocklist:          inGame,
	cmCheckNickname:          inAuthed,
	cmReplaceItem:            inGame,
	cmBlockSetReason:         inGame,
	cmMacAddress2:            inGame,
	cmMacroCreate:            inGame,
	cmMacroDelete:            inGame,
	cmDistributionSettings:   inGame,
	cmMayLoginIntoGame:       inAuthed,
	cmShowBrand:              inGame,
	cmReconnectAuth:          inAuthed,
	cmGroupLoot:              inGame,
	cmShowMap:                inGame,
	cmMacAddress:             inConnected | inAuthed | inGame,
	cmReportPlayer:           inGame,
	cmGroupResponse:          inGame,
	cmSummonMove:             inGame,
	cmSummonEmotion:          inGame,
	cmSummonAttack:           inGame,
	cmDeleteQuest:            inGame,
	cmItemRemodel:            inGame,
	cmGodstoneSocket:         inGame,
	cmInviteToGroup:          inGame,
	cmAllianceGroupChange:    inGame,
	cmViewPlayerDetails:      inGame,
	cmPlayerStatusInfo:       inGame,
	cmClientCommandRoll:      inGame,
	cmGroupDistribution:      inGame,
	cmPingRequest:            inGame,
	cmDuelRequest:            inGame,
	cmDeleteItem:             inGame,
	cmShowFriendlist:         inGame,
	cmFriendAdd:              inGame,
	cmFriendDel:              inGame,
	cmSummonCommand:          inGame,
	cmBrokerList:             inGame,
	cmPrivateStore:           inGame,
	cmPrivateStoreName:       inGame,
	cmBrokerSettleList:       inGame,
	cmBrokerSettleAccount:    inGame,
	cmSendMail:               inGame,
	cmBrokerRegistered:       inGame,
	cmBuyBrokerItem:          inGame,
	cmRegisterBrokerItem:     inGame,
	cmBrokerCancelRegistered: inGame,
	cmDeleteMail:             inGame,
	cmTitleSet:               inGame,
	cmReadMail:               inGame,
	cmGetMailAttachment:      inGame,
	cmTeleportSelect:         inGame,
	cmPetition:               inGame,
	cmChatMessagePublic:      inGame,
	cmChatMessageWhisper:     inGame,
	cmOpenStaticdoor:         inGame,
	cmCastspell:              inGame,
	cmSkillDeactivate:        inGame,
	cmRemoveAlteredState:     inGame,
	cmTargetSelect:           inGame,
	cmAttack:                 inGame,
	cmEmotion:                inGame,
	cmPing:                   inAuthed | inGame,
	cmUseItem:                inGame,
	cmEquipItem:              inGame,
	cmFlightTeleport:         inGame,
	cmQuestionResponse:       inGame,
	cmBuyItem:                inGame,
	cmShowDialog:             inGame,
	cmLegion:                 inGame,
	cmMove:                   inGame,
	cmSetNote:                inGame,
	cmLegionModifyEmblem:     inGame,
	cmCloseDialog:            inGame,
	cmDialogSelect:           inGame,
	cmLegionTabs:             inGame,
	cmExchangeAddKinah:       inGame,
	cmExchangeLock:           inGame,
	cmExchangeOk:             inGame,
	cmExchangeRequest:        inGame,
	cmExchangeAddItem:        inGame,
	cmManastone:              inGame,
	cmExchangeCancel:         inGame,
	cmPlayMovieEnd:           inGame,
	cmSummonCastspell:        inGame,
	cmFusionWeapons:          inGame,
	cmBreakWeapons:           inGame,
	cmLegionSendEmblem:       inGame,
	cmDisconnect:             inGame,
	cmQuit:                   inAuthed | inGame,
	cmMayQuit:                inAuthed | inGame,
	cmVersionCheck:           inConnected,
	cmLevelReady:             inGame,
	cmUiSettings:             inGame,
	cmObjectSearch:           inGame,
	cmCustomSettings:         inGame,
	cmRevive:                 inGame,
	cmEnterWorld:             inAuthed,
	cmTimeCheck:              inConnected | inAuthed | inGame,
	cmGather:                 inGame,
	cmQuestionnaire:          inGame,
}

// serverNames and clientNames name the opcodes, for logs.
var serverNames = map[byte]string{
	smStatupdateMp:              "SM_STATUPDATE_MP",
	smStatupdateHp:              "SM_STATUPDATE_HP",
	smChatInit:                  "SM_CHAT_INIT",
	smChannelInfo:               "SM_CHANNEL_INFO",
	smMacroResult:               "SM_MACRO_RESULT",
	smMacroList:                 "SM_MACRO_LIST",
	smNicknameCheckResponse:     "SM_NICKNAME_CHECK_RESPONSE",
	smRiftAnnounce:              "SM_RIFT_ANNOUNCE",
	smSetBindPoint:              "SM_SET_BIND_POINT",
	smAbyssRank:                 "SM_ABYSS_RANK",
	smFriendUpdate:              "SM_FRIEND_UPDATE",
	smPetition:                  "SM_PETITION",
	smRecipeDelete:              "SM_RECIPE_DELETE",
	smLearnRecipe:               "SM_LEARN_RECIPE",
	smFlyTime:                   "SM_FLY_TIME",
	smDelete:                    "SM_DELETE",
	smPlayerMove:                "SM_PLAYER_MOVE",
	smMessage:                   "SM_MESSAGE",
	smLoginQueue:                "SM_LOGIN_QUEUE",
	smInventoryInfo:             "SM_INVENTORY_INFO",
	smSystemMessage:             "SM_SYSTEM_MESSAGE",
	smDeleteItem:                "SM_DELETE_ITEM",
	smAddItems:                  "SM_ADD_ITEMS",
	smUiSettings:                "SM_UI_SETTINGS",
	smUpdateItem:                "SM_UPDATE_ITEM",
	smPlayerInfo:                "SM_PLAYER_INFO",
	smGatherStatus:              "SM_GATHER_STATUS",
	smCastspell:                 "SM_CASTSPELL",
	smUpdatePlayerAppearance:    "SM_UPDATE_PLAYER_APPEARANCE",
	smGatherUpdate:              "SM_GATHER_UPDATE",
	smStatupdateDp:              "SM_STATUPDATE_DP",
	smAttackStatus:              "SM_ATTACK_STATUS",
	smStatupdateExp:             "SM_STATUPDATE_EXP",
	smDpInfo:                    "SM_DP_INFO",
	smLegionTabs:                "SM_LEGION_TABS",
	smLegionUpdateNickname:      "SM_LEGION_UPDATE_NICKNAME",
	smNpcInfo:                   "SM_NPC_INFO",
	smEnterWorldCheck:           "SM_ENTER_WORLD_CHECK",
	smPlayerSpawn:               "SM_PLAYER_SPAWN",
	smGatherableInfo:            "SM_GATHERABLE_INFO",
	smTeleportLoc:               "SM_TELEPORT_LOC",
	smAttack:                    "SM_ATTACK",
	smMove:                      "SM_MOVE",
	smTransform:                 "SM_TRANSFORM",
	smDialogWindow:              "SM_DIALOG_WINDOW",
	smSellItem:                  "SM_SELL_ITEM",
	smViewPlayerDetails:         "SM_VIEW_PLAYER_DETAILS",
	smPlayerState:               "SM_PLAYER_STATE",
	smWeather:                   "SM_WEATHER",
	smGameTime:                  "SM_GAME_TIME",
	smEmotion:                   "SM_EMOTION",
	smLookatobject:              "SM_LOOKATOBJECT",
	smTimeCheck:                 "SM_TIME_CHECK",
	smSkillCancel:               "SM_SKILL_CANCEL",
	smTargetSelected:            "SM_TARGET_SELECTED",
	smSkillList:                 "SM_SKILL_LIST",
	smCastspellEnd:              "SM_CASTSPELL_END",
	smSkillActivation:           "SM_SKILL_ACTIVATION",
	smSkillRemove:               "SM_SKILL_REMOVE",
	smAbnormalEffect:            "SM_ABNORMAL_EFFECT",
	smAbnormalState:             "SM_ABNORMAL_STATE",
	smQuestionWindow:            "SM_QUESTION_WINDOW",
	smSkillCooldown:             "SM_SKILL_COOLDOWN",
	smInfluenceRatio:            "SM_INFLUENCE_RATIO",
	smNameChange:                "SM_NAME_CHANGE",
	smGroupInfo:                 "SM_GROUP_INFO",
	smShowNpcOnMap:              "SM_SHOW_NPC_ON_MAP",
	smGroupMemberInfo:           "SM_GROUP_MEMBER_INFO",
	smQuitResponse:              "SM_QUIT_RESPONSE",
	smLevelUpdate:               "SM_LEVEL_UPDATE",
	smKey:                       "SM_KEY",
	smExchangeRequest:           "SM_EXCHANGE_REQUEST",
	smSummonPanelRemove:         "SM_SUMMON_PANEL_REMOVE",
	smExchangeAddItem:           "SM_EXCHANGE_ADD_ITEM",
	smExchangeConfirmation:      "SM_EXCHANGE_CONFIRMATION",
	smExchangeAddKinah:          "SM_EXCHANGE_ADD_KINAH",
	smEmotionList:               "SM_EMOTION_LIST",
	smTargetUpdate:              "SM_TARGET_UPDATE",
	smLegionUpdateSelfIntro:     "SM_LEGION_UPDATE_SELF_INTRO",
	smRiftStatus:                "SM_RIFT_STATUS",
	smQuestAccepted:             "SM_QUEST_ACCEPTED",
	smQuestList:                 "SM_QUEST_LIST",
	smPingResponse:              "SM_PING_RESPONSE",
	smNearbyQuests:              "SM_NEARBY_QUESTS",
	smCubeUpdate:                "SM_CUBE_UPDATE",
	smFriendList:                "SM_FRIEND_LIST",
	smUpdateNote:                "SM_UPDATE_NOTE",
	smItemCooldown:              "SM_ITEM_COOLDOWN",
	smPlayMovie:                 "SM_PLAY_MOVIE",
	smLegionInfo:                "SM_LEGION_INFO",
	smLegionLeaveMember:         "SM_LEGION_LEAVE_MEMBER",
	smLegionAddMember:           "SM_LEGION_ADD_MEMBER",
	smLegionUpdateTitle:         "SM_LEGION_UPDATE_TITLE",
	smLegionUpdateMember:        "SM_LEGION_UPDATE_MEMBER",
	smBrokerRegistrationService: "SM_BROKER_REGISTRATION_SERVICE",
	smBrokerSettledList:         "SM_BROKER_SETTLED_LIST",
	smSummonOwnerRemove:         "SM_SUMMON_OWNER_REMOVE",
	smSummonPanel:               "SM_SUMMON_PANEL",
	smSummonUpdate:              "SM_SUMMON_UPDATE",
	smLegionEdit:                "SM_LEGION_EDIT",
	smLegionMemberlist:          "SM_LEGION_MEMBERLIST",
	smSummonUseskill:            "SM_SUMMON_USESKILL",
	smMailService:               "SM_MAIL_SERVICE",
	smPrivateStore:              "SM_PRIVATE_STORE",
	smAbyssRankUpdate:           "SM_ABYSS_RANK_UPDATE",
	smGroupLoot:                 "SM_GROUP_LOOT",
	smMayLoginIntoGame:          "SM_MAY_LOGIN_INTO_GAME",
	smPong:                      "SM_PONG",
	smPlayerId:                  "SM_PLAYER_ID",
	smKiskUpdate:                "SM_KISK_UPDATE",
	smBrokerItems:               "SM_BROKER_ITEMS",
	smPrivateStoreName:          "SM_PRIVATE_STORE_NAME",
	smBrokerRegisteredList:      "SM_BROKER_REGISTERED_LIST",
	smAscensionMorph:            "SM_ASCENSION_MORPH",
	smCraftUpdate:               "SM_CRAFT_UPDATE",
	smCustomSettings:            "SM_CUSTOM_SETTINGS",
	smItemUsageAnimation:        "SM_ITEM_USAGE_ANIMATION",
	smDuel:                      "SM_DUEL",
	smResurrect:                 "SM_RESURRECT",
	smDie:                       "SM_DIE",
	smTeleportMap:               "SM_TELEPORT_MAP",
	smForcedMove:                "SM_FORCED_MOVE",
	smWarehouseInfo:             "SM_WAREHOUSE_INFO",
	smDeleteWarehouseItem:       "SM_DELETE_WAREHOUSE_ITEM",
	smWarehouseUpdate:           "SM_WAREHOUSE_UPDATE",
	smUpdateWarehouseItem:       "SM_UPDATE_WAREHOUSE_ITEM",
	smTitleList:                 "SM_TITLE_LIST",
	smTitleSet:                  "SM_TITLE_SET",
	smCraftAnimation:            "SM_CRAFT_ANIMATION",
	smTitleUpdate:               "SM_TITLE_UPDATE",
	smLegionSendEmblem:          "SM_LEGION_SEND_EMBLEM",
	smLegionUpdateEmblem:        "SM_LEGION_UPDATE_EMBLEM",
	smFriendResponse:            "SM_FRIEND_RESPONSE",
	smBlockList:                 "SM_BLOCK_LIST",
	smBlockResponse:             "SM_BLOCK_RESPONSE",
	smFriendNotify:              "SM_FRIEND_NOTIFY",
	smUseObject:                 "SM_USE_OBJECT",
	smCharacterList:             "SM_CHARACTER_LIST",
	smL2authLoginCheck:          "SM_L2AUTH_LOGIN_CHECK",
	smDeleteCharacter:           "SM_DELETE_CHARACTER",
	smCreateCharacter:           "SM_CREATE_CHARACTER",
	smTargetImmobilize:          "SM_TARGET_IMMOBILIZE",
	smRestoreCharacter:          "SM_RESTORE_CHARACTER",
	smLootItemlist:              "SM_LOOT_ITEMLIST",
	smLootStatus:                "SM_LOOT_STATUS",
	smMantraEffect:              "SM_MANTRA_EFFECT",
	smRecipeList:                "SM_RECIPE_LIST",
	smSiegeLocationInfo:         "SM_SIEGE_LOCATION_INFO",
	smPlayerSearch:              "SM_PLAYER_SEARCH",
	smAllianceMemberInfo:        "SM_ALLIANCE_MEMBER_INFO",
	smAllianceInfo:              "SM_ALLIANCE_INFO",
	smLeaveGroupMember:          "SM_LEAVE_GROUP_MEMBER",
	smAllianceReadyCheck:        "SM_ALLIANCE_READY_CHECK",
	smShowBrand:                 "SM_SHOW_BRAND",
	smPrices:                    "SM_PRICES",
	smTradelist:                 "SM_TRADELIST",
	smVersionCheck:              "SM_VERSION_CHECK",
	smReconnectKey:              "SM_RECONNECT_KEY",
	smStatsInfo:                 "SM_STATS_INFO",
	smQuestionnaire:             "SM_QUESTIONNAIRE",
}

var clientNames = map[byte]string{
	cmCraft:                  "CM_CRAFT",
	cmClientCommandLoc:       "CM_CLIENT_COMMAND_LOC",
	cmRestoreCharacter:       "CM_RESTORE_CHARACTER",
	cmStartLoot:              "CM_START_LOOT",
	cmLootItem:               "CM_LOOT_ITEM",
	cmMoveItem:               "CM_MOVE_ITEM",
	cmL2authLoginCheck:       "CM_L2AUTH_LOGIN_CHECK",
	cmCharacterList:          "CM_CHARACTER_LIST",
	cmCreateCharacter:        "CM_CREATE_CHARACTER",
	cmDeleteCharacter:        "CM_DELETE_CHARACTER",
	cmLegionUploadEmblem:     "CM_LEGION_UPLOAD_EMBLEM",
	cmSplitItem:              "CM_SPLIT_ITEM",
	cmPlayerSearch:           "CM_PLAYER_SEARCH",
	cmLegionUploadInfo:       "CM_LEGION_UPLOAD_INFO",
	cmFriendStatus:           "CM_FRIEND_STATUS",
	cmChangeChannel:          "CM_CHANGE_CHANNEL",
	cmBlockAdd:               "CM_BLOCK_ADD",
	cmBlockDel:               "CM_BLOCK_DEL",
	cmShowBlocklist:          "CM_SHOW_BLOCKLIST",
	cmCheckNickname:          "CM_CHECK_NICKNAME",
	cmReplaceItem:            "CM_REPLACE_ITEM",
	cmBlockSetReason:         "CM_BLOCK_SET_REASON",
	cmMacAddress2:            "CM_MAC_ADDRESS2",
	cmMacroCreate:            "CM_MACRO_CREATE",
	cmMacroDelete:            "CM_MACRO_DELETE",
	cmDistributionSettings:   "CM_DISTRIBUTION_SETTINGS",
	cmMayLoginIntoGame:       "CM_MAY_LOGIN_INTO_GAME",
	cmShowBrand:              "CM_SHOW_BRAND",
	cmReconnectAuth:          "CM_RECONNECT_AUTH",
	cmGroupLoot:              "CM_GROUP_LOOT",
	cmShowMap:                "CM_SHOW_MAP",
	cmMacAddress:             "CM_MAC_ADDRESS",
	cmReportPlayer:           "CM_REPORT_PLAYER",
	cmGroupResponse:          "CM_GROUP_RESPONSE",
	cmSummonMove:             "CM_SUMMON_MOVE",
	cmSummonEmotion:          "CM_SUMMON_EMOTION",
	cmSummonAttack:           "CM_SUMMON_ATTACK",
	cmDeleteQuest:            "CM_DELETE_QUEST",
	cmItemRemodel:            "CM_ITEM_REMODEL",
	cmGodstoneSocket:         "CM_GODSTONE_SOCKET",
	cmInviteToGroup:          "CM_INVITE_TO_GROUP",
	cmAllianceGroupChange:    "CM_ALLIANCE_GROUP_CHANGE",
	cmViewPlayerDetails:      "CM_VIEW_PLAYER_DETAILS",
	cmPlayerStatusInfo:       "CM_PLAYER_STATUS_INFO",
	cmClientCommandRoll:      "CM_CLIENT_COMMAND_ROLL",
	cmGroupDistribution:      "CM_GROUP_DISTRIBUTION",
	cmPingRequest:            "CM_PING_REQUEST",
	cmDuelRequest:            "CM_DUEL_REQUEST",
	cmDeleteItem:             "CM_DELETE_ITEM",
	cmShowFriendlist:         "CM_SHOW_FRIENDLIST",
	cmFriendAdd:              "CM_FRIEND_ADD",
	cmFriendDel:              "CM_FRIEND_DEL",
	cmSummonCommand:          "CM_SUMMON_COMMAND",
	cmBrokerList:             "CM_BROKER_LIST",
	cmPrivateStore:           "CM_PRIVATE_STORE",
	cmPrivateStoreName:       "CM_PRIVATE_STORE_NAME",
	cmBrokerSettleList:       "CM_BROKER_SETTLE_LIST",
	cmBrokerSettleAccount:    "CM_BROKER_SETTLE_ACCOUNT",
	cmSendMail:               "CM_SEND_MAIL",
	cmBrokerRegistered:       "CM_BROKER_REGISTERED",
	cmBuyBrokerItem:          "CM_BUY_BROKER_ITEM",
	cmRegisterBrokerItem:     "CM_REGISTER_BROKER_ITEM",
	cmBrokerCancelRegistered: "CM_BROKER_CANCEL_REGISTERED",
	cmDeleteMail:             "CM_DELETE_MAIL",
	cmTitleSet:               "CM_TITLE_SET",
	cmReadMail:               "CM_READ_MAIL",
	cmGetMailAttachment:      "CM_GET_MAIL_ATTACHMENT",
	cmTeleportSelect:         "CM_TELEPORT_SELECT",
	cmPetition:               "CM_PETITION",
	cmChatMessagePublic:      "CM_CHAT_MESSAGE_PUBLIC",
	cmChatMessageWhisper:     "CM_CHAT_MESSAGE_WHISPER",
	cmOpenStaticdoor:         "CM_OPEN_STATICDOOR",
	cmCastspell:              "CM_CASTSPELL",
	cmSkillDeactivate:        "CM_SKILL_DEACTIVATE",
	cmRemoveAlteredState:     "CM_REMOVE_ALTERED_STATE",
	cmTargetSelect:           "CM_TARGET_SELECT",
	cmAttack:                 "CM_ATTACK",
	cmEmotion:                "CM_EMOTION",
	cmPing:                   "CM_PING",
	cmUseItem:                "CM_USE_ITEM",
	cmEquipItem:              "CM_EQUIP_ITEM",
	cmFlightTeleport:         "CM_FLIGHT_TELEPORT",
	cmQuestionResponse:       "CM_QUESTION_RESPONSE",
	cmBuyItem:                "CM_BUY_ITEM",
	cmShowDialog:             "CM_SHOW_DIALOG",
	cmLegion:                 "CM_LEGION",
	cmMove:                   "CM_MOVE",
	cmSetNote:                "CM_SET_NOTE",
	cmLegionModifyEmblem:     "CM_LEGION_MODIFY_EMBLEM",
	cmCloseDialog:            "CM_CLOSE_DIALOG",
	cmDialogSelect:           "CM_DIALOG_SELECT",
	cmLegionTabs:             "CM_LEGION_TABS",
	cmExchangeAddKinah:       "CM_EXCHANGE_ADD_KINAH",
	cmExchangeLock:           "CM_EXCHANGE_LOCK",
	cmExchangeOk:             "CM_EXCHANGE_OK",
	cmExchangeRequest:        "CM_EXCHANGE_REQUEST",
	cmExchangeAddItem:        "CM_EXCHANGE_ADD_ITEM",
	cmManastone:              "CM_MANASTONE",
	cmExchangeCancel:         "CM_EXCHANGE_CANCEL",
	cmPlayMovieEnd:           "CM_PLAY_MOVIE_END",
	cmSummonCastspell:        "CM_SUMMON_CASTSPELL",
	cmFusionWeapons:          "CM_FUSION_WEAPONS",
	cmBreakWeapons:           "CM_BREAK_WEAPONS",
	cmLegionSendEmblem:       "CM_LEGION_SEND_EMBLEM",
	cmDisconnect:             "CM_DISCONNECT",
	cmQuit:                   "CM_QUIT",
	cmMayQuit:                "CM_MAY_QUIT",
	cmVersionCheck:           "CM_VERSION_CHECK",
	cmLevelReady:             "CM_LEVEL_READY",
	cmUiSettings:             "CM_UI_SETTINGS",
	cmObjectSearch:           "CM_OBJECT_SEARCH",
	cmCustomSettings:         "CM_CUSTOM_SETTINGS",
	cmRevive:                 "CM_REVIVE",
	cmEnterWorld:             "CM_ENTER_WORLD",
	cmTimeCheck:              "CM_TIME_CHECK",
	cmGather:                 "CM_GATHER",
	cmQuestionnaire:          "CM_QUESTIONNAIRE",
}
