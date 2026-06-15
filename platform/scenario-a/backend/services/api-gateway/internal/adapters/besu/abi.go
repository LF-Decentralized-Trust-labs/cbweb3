package besu

// htlcEventsABI contains only the event definitions needed for FilterLogs.
const htlcEventsABI = `[
  {
    "anonymous": false,
    "inputs": [
      {"indexed": true,  "name": "contractId",  "type": "bytes32"},
      {"indexed": true,  "name": "sender",      "type": "address"},
      {"indexed": true,  "name": "receiver",    "type": "address"},
      {"indexed": false, "name": "hashLock",    "type": "bytes32"},
      {"indexed": false, "name": "timeLock",    "type": "uint256"},
      {"indexed": false, "name": "zetoLockRef", "type": "bytes32"}
    ],
    "name": "LogHTLCLocked",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {"indexed": true,  "name": "contractId", "type": "bytes32"},
      {"indexed": false, "name": "secret",     "type": "bytes32"}
    ],
    "name": "LogHTLCClaimed",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {"indexed": true, "name": "contractId", "type": "bytes32"}
    ],
    "name": "LogHTLCRefunded",
    "type": "event"
  }
]`
