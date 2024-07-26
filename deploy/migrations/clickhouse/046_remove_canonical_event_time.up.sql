DROP TABLE IF EXISTS default.canonical_beacon_blob_sidecar ON CLUSTER '{cluster}' SYNC;

DROP TABLE IF EXISTS default.canonical_beacon_block ON CLUSTER '{cluster}' SYNC;

DROP TABLE IF EXISTS default.canonical_beacon_block_attester_slashing ON CLUSTER '{cluster}' SYNC;

DROP TABLE IF EXISTS default.canonical_beacon_block_bls_to_execution_change ON CLUSTER '{cluster}' SYNC;

DROP TABLE IF EXISTS default.canonical_beacon_block_deposit ON CLUSTER '{cluster}' SYNC;

DROP TABLE IF EXISTS default.canonical_beacon_block_execution_transaction ON CLUSTER '{cluster}' SYNC;

DROP TABLE IF EXISTS default.canonical_beacon_block_proposer_slashing ON CLUSTER '{cluster}' SYNC;

DROP TABLE IF EXISTS default.canonical_beacon_block_voluntary_exit ON CLUSTER '{cluster}' SYNC;

DROP TABLE IF EXISTS default.canonical_beacon_block_withdrawal ON CLUSTER '{cluster}' SYNC;

DROP TABLE IF EXISTS default.canonical_beacon_elaborated_attestation ON CLUSTER '{cluster}' SYNC;

DROP TABLE IF EXISTS default.canonical_beacon_proposer_duty ON CLUSTER '{cluster}' SYNC;

DROP TABLE IF EXISTS default.canonical_beacon_validators ON CLUSTER '{cluster}' SYNC;

DROP TABLE IF EXISTS default.canonical_beacon_validators_pubkeys ON CLUSTER '{cluster}' SYNC;

DROP TABLE IF EXISTS default.canonical_beacon_validators_withdrawal_credentials ON CLUSTER '{cluster}' SYNC;

DROP TABLE IF EXISTS default.canonical_beacon_committee ON CLUSTER '{cluster}' SYNC;

ALTER TABLE
    default.canonical_beacon_blob_sidecar_local ON CLUSTER '{cluster}' DROP COLUMN event_date_time;

ALTER TABLE
    default.canonical_beacon_block_local ON CLUSTER '{cluster}' DROP COLUMN event_date_time;

ALTER TABLE
    default.canonical_beacon_block_attester_slashing_local ON CLUSTER '{cluster}' DROP COLUMN event_date_time;

ALTER TABLE
    default.canonical_beacon_block_bls_to_execution_change_local ON CLUSTER '{cluster}' DROP COLUMN event_date_time;

ALTER TABLE
    default.canonical_beacon_block_deposit_local ON CLUSTER '{cluster}' DROP COLUMN event_date_time;

ALTER TABLE
    default.canonical_beacon_block_execution_transaction_local ON CLUSTER '{cluster}' DROP COLUMN event_date_time;

ALTER TABLE
    default.canonical_beacon_block_proposer_slashing_local ON CLUSTER '{cluster}' DROP COLUMN event_date_time;

ALTER TABLE
    default.canonical_beacon_block_voluntary_exit_local ON CLUSTER '{cluster}' DROP COLUMN event_date_time;

ALTER TABLE
    default.canonical_beacon_block_withdrawal_local ON CLUSTER '{cluster}' DROP COLUMN event_date_time;

ALTER TABLE
    default.canonical_beacon_elaborated_attestation_local ON CLUSTER '{cluster}' DROP COLUMN event_date_time;

ALTER TABLE
    default.canonical_beacon_proposer_duty_local ON CLUSTER '{cluster}' DROP COLUMN event_date_time;

ALTER TABLE
    default.canonical_beacon_validators_local ON CLUSTER '{cluster}' DROP COLUMN event_date_time;

ALTER TABLE
    default.canonical_beacon_validators_pubkeys_local ON CLUSTER '{cluster}' DROP COLUMN event_date_time;

ALTER TABLE
    default.canonical_beacon_validators_withdrawal_credentials_local ON CLUSTER '{cluster}' DROP COLUMN event_date_time;

ALTER TABLE
    default.canonical_beacon_committee_local ON CLUSTER '{cluster}' DROP COLUMN event_date_time;

CREATE TABLE default.canonical_beacon_blob_sidecar ON CLUSTER '{cluster}' AS default.canonical_beacon_blob_sidecar_local ENGINE = Distributed(
    '{cluster}',
    default,
    canonical_beacon_blob_sidecar_local,
    cityHash64(
        slot_start_date_time,
        meta_network_name,
        block_root,
        blob_index
    )
) COMMENT 'Contains a blob sidecar from a beacon block.';

CREATE TABLE default.canonical_beacon_block ON CLUSTER '{cluster}' AS default.canonical_beacon_block_local ENGINE = Distributed(
    '{cluster}',
    default,
    canonical_beacon_block_local,
    cityHash64(
        slot_start_date_time,
        meta_network_name
    )
) COMMENT 'Contains beacon block from a beacon node.';

CREATE TABLE default.canonical_beacon_block_attester_slashing ON CLUSTER '{cluster}' AS default.canonical_beacon_block_attester_slashing_local ENGINE = Distributed(
    '{cluster}',
    default,
    canonical_beacon_block_attester_slashing_local,
    cityHash64(
        slot_start_date_time,
        meta_network_name,
        block_root,
        attestation_1_attesting_indices,
        attestation_2_attesting_indices,
        attestation_1_data_slot,
        attestation_2_data_slot,
        attestation_1_data_beacon_block_root,
        attestation_2_data_beacon_block_root
    )
) COMMENT 'Contains attester slashing from a beacon block.';

CREATE TABLE default.canonical_beacon_block_bls_to_execution_change ON CLUSTER '{cluster}' AS default.canonical_beacon_block_bls_to_execution_change_local ENGINE = Distributed(
    '{cluster}',
    default,
    canonical_beacon_block_bls_to_execution_change_local,
    cityHash64(
        slot_start_date_time,
        meta_network_name,
        block_root,
        exchanging_message_validator_index,
        exchanging_message_from_bls_pubkey,
        exchanging_message_to_execution_address
    )
) COMMENT 'Contains bls to execution change from a beacon block.';

CREATE TABLE default.canonical_beacon_block_deposit ON CLUSTER '{cluster}' AS default.canonical_beacon_block_deposit_local ENGINE = Distributed(
    '{cluster}',
    default,
    canonical_beacon_block_deposit_local,
    cityHash64(
        slot_start_date_time,
        meta_network_name,
        block_root,
        deposit_data_pubkey,
        deposit_proof
    )
) COMMENT 'Contains a deposit from a beacon block.';

CREATE TABLE default.canonical_beacon_block_execution_transaction ON CLUSTER '{cluster}' AS default.canonical_beacon_block_execution_transaction_local ENGINE = Distributed(
    '{cluster}',
    default,
    canonical_beacon_block_execution_transaction_local,
    cityHash64(
        slot_start_date_time,
        meta_network_name,
        block_root,
        position,
        hash,
        nonce
    )
) COMMENT 'Contains execution transaction from a beacon block.';

CREATE TABLE default.canonical_beacon_block_proposer_slashing ON CLUSTER '{cluster}' AS default.canonical_beacon_block_proposer_slashing_local ENGINE = Distributed(
    '{cluster}',
    default,
    canonical_beacon_block_proposer_slashing_local,
    cityHash64(
        slot_start_date_time,
        meta_network_name,
        block_root,
        signed_header_1_message_slot,
        signed_header_2_message_slot,
        signed_header_1_message_proposer_index,
        signed_header_2_message_proposer_index,
        signed_header_1_message_body_root,
        signed_header_2_message_body_root
    )
) COMMENT 'Contains proposer slashing from a beacon block.';

CREATE TABLE default.canonical_beacon_block_voluntary_exit ON CLUSTER '{cluster}' AS default.canonical_beacon_block_voluntary_exit_local ENGINE = Distributed(
    '{cluster}',
    default,
    canonical_beacon_block_voluntary_exit_local,
    cityHash64(
        slot_start_date_time,
        meta_network_name,
        block_root,
        voluntary_exit_message_epoch,
        voluntary_exit_message_validator_index
    )
) COMMENT 'Contains a voluntary exit from a beacon block.';

CREATE TABLE default.canonical_beacon_block_withdrawal ON CLUSTER '{cluster}' AS default.canonical_beacon_block_withdrawal_local ENGINE = Distributed(
    '{cluster}',
    default,
    canonical_beacon_block_withdrawal_local,
    cityHash64(
        slot_start_date_time,
        meta_network_name,
        block_root,
        withdrawal_index,
        withdrawal_validator_index
    )
) COMMENT 'Contains a withdrawal from a beacon block.';

CREATE TABLE default.canonical_beacon_elaborated_attestation ON CLUSTER '{cluster}' AS default.canonical_beacon_elaborated_attestation_local ENGINE = Distributed(
    '{cluster}',
    default,
    canonical_beacon_elaborated_attestation_local,
    cityHash64(
        slot_start_date_time,
        meta_network_name,
        block_root,
        block_slot,
        position_in_block,
        beacon_block_root,
        slot,
        committee_index,
        source_root,
        target_root
    )
) COMMENT 'Contains elaborated attestations from beacon blocks.';

CREATE TABLE default.canonical_beacon_proposer_duty ON CLUSTER '{cluster}' AS default.canonical_beacon_proposer_duty_local ENGINE = Distributed(
    '{cluster}',
    default,
    canonical_beacon_proposer_duty_local,
    cityHash64(
        slot_start_date_time,
        meta_network_name,
        proposer_validator_index,
        proposer_pubkey
    )
) COMMENT 'Contains a proposer duty from a beacon block.';

CREATE TABLE default.canonical_beacon_validators ON CLUSTER '{cluster}' AS default.canonical_beacon_validators_local ENGINE = Distributed(
    '{cluster}',
    default,
    canonical_beacon_validators_local,
    cityHash64(
        epoch_start_date_time,
        meta_network_name,
        `index`,
        `status`
    )
) COMMENT 'Contains a validator state for an epoch.';

CREATE TABLE default.canonical_beacon_validators_pubkeys on cluster '{cluster}' AS default.canonical_beacon_validators_pubkeys_local ENGINE = Distributed(
    '{cluster}',
    default,
    canonical_beacon_validators_pubkeys_local,
    cityHash64(`index`, meta_network_name)
) COMMENT 'Contains a validator pubkeys for an epoch.';

CREATE TABLE default.canonical_beacon_validators_withdrawal_credentials on cluster '{cluster}' AS default.canonical_beacon_validators_withdrawal_credentials_local ENGINE = Distributed(
    '{cluster}',
    default,
    canonical_beacon_validators_withdrawal_credentials_local,
    cityHash64(`index`, meta_network_name)
) COMMENT 'Contains a validator withdrawal credentials for an epoch.';

CREATE TABLE default.canonical_beacon_committee ON CLUSTER '{cluster}' AS default.canonical_beacon_committee_local ENGINE = Distributed(
    '{cluster}',
    default,
    canonical_beacon_committee_local,
    cityHash64(
        slot_start_date_time,
        meta_network_name,
        committee_index
    )
) COMMENT 'Contains canonical beacon API /eth/v1/beacon/committees data.';
