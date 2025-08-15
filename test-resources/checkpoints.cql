create table nexus.checkpoints
(
    algorithm                 text,
    id                        text,
    lifecycle_stage           text,
    payload_uri               text,
    result_uri                text,
    algorithm_failure_cause   text,
    algorithm_failure_details text,
    received_by_host          text,
    received_at               timestamp,
    sent_at                   timestamp,
    applied_configuration     text,
    configuration_overrides   text,
    content_hash              text,
    last_modified             timestamp,
    tag                       text,
    api_version               text,
    job_uid                   text,
    parent_job                text,
    payload_valid_for         text,
    PRIMARY KEY ((algorithm, id))
);

create index submission_tag ON nexus.checkpoints (tag);

create index host ON nexus.checkpoints (received_by_host);

create index lifecycle_stage ON nexus.checkpoints (lifecycle_stage);

INSERT INTO nexus.checkpoints (algorithm, id, lifecycle_stage, payload_uri, result_uri, algorithm_failure_cause,
                               algorithm_failure_details, received_by_host, received_at, sent_at, applied_configuration,
                               configuration_overrides, content_hash, last_modified, tag, api_version,
                               job_uid, parent_job, payload_valid_for)
VALUES ('test-algorithm', 'f47ac10b-58cc-4372-a567-0e02b2c3d479', 'BUFFERED', 'http://localhost/payload',
        NULL, NULL, NULL, 'host123', '2023-10-01T12:00:00.000Z',
        '2023-10-01T12:30:00.000Z', '{}', '{}', 'new_hash', '2023-10-01T12:45:00.000Z',
        'tag_123', 'v1.0', 'd94c16c8-2c1e-4f3a-85d1-2d9c3b7f0a24', '{}', '1h');

INSERT INTO nexus.checkpoints (algorithm, id, lifecycle_stage, payload_uri, result_uri, algorithm_failure_cause,
                               algorithm_failure_details, received_by_host, received_at, sent_at, applied_configuration,
                               configuration_overrides, content_hash, last_modified, tag, api_version,
                               job_uid, parent_job, payload_valid_for)
VALUES ('test-algorithm', '2c7b6e8d-cc3c-4b5b-a3f6-5d7b9e2c7f2a', 'RUNNING', 'http://localhost/payload',
        NULL, NULL, NULL, 'host123', '2023-10-02T10:00:00.000Z',
        '2023-10-02T10:30:00.000Z', '{}', '{}', 'buffered_hash', '2023-10-02T10:45:00.000Z',
        'tag_123', 'v2.0', '1f7b6e8d-cc3c-4b5b-a3f6-5d7b9e2c7f2a', '{}', '15m');

INSERT INTO nexus.checkpoints (algorithm, id, lifecycle_stage, payload_uri, result_uri, algorithm_failure_cause,
                               algorithm_failure_details, received_by_host, received_at, sent_at, applied_configuration,
                               configuration_overrides, content_hash, last_modified, tag, api_version,
                               job_uid, parent_job, payload_valid_for)
VALUES ('test-algorithm', '3c7b6e8d-cc3c-4b5b-a3f6-5d7b9e2c7f2b', 'RUNNING', 'http://localhost/payload',
        NULL, NULL, NULL, 'host123', '2023-10-02T10:00:00.000Z',
        '2023-10-02T10:30:00.000Z', '{}', '{}', 'buffered_hash', '2023-10-02T10:45:00.000Z',
        'tag_123', 'v2.0', '1f7b6e8d-cc3c-4b5b-a3f6-5d7b9e2c7f2a', '{}', '15m');

INSERT INTO nexus.checkpoints (algorithm, id, lifecycle_stage, payload_uri, result_uri, algorithm_failure_cause,
                               algorithm_failure_details, received_by_host, received_at, sent_at, applied_configuration,
                               configuration_overrides, content_hash, last_modified, tag, api_version,
                               job_uid, parent_job, payload_valid_for)
VALUES ('test-algorithm', '4c7b6e8d-cc3c-fb5b-a3f6-5d7b9e2c7f2b', 'BUFFERED', 'http://localhost/payload',
        NULL, NULL, NULL, 'host123', '2023-10-02T10:00:00.000Z',
        '2023-10-02T10:30:00.000Z', '{}', '{}', 'buffered_hash', '2023-10-02T10:45:00.000Z',
        'tag_123', 'v2.0', '1f7b6e8d-cc3c-4b5b-a3f6-5d7b9e2c7f2a', '{}', '15m');

INSERT INTO nexus.checkpoints (algorithm, id, lifecycle_stage, payload_uri, result_uri, algorithm_failure_cause,
                               algorithm_failure_details, received_by_host, received_at, sent_at, applied_configuration,
                               configuration_overrides, content_hash, last_modified, tag, api_version,
                               job_uid, parent_job, payload_valid_for)
VALUES ('test-algorithm', '1d7b6e8d-cc3c-fb5b-a3f6-5d7b9e2c7f2b', 'RUNNING', 'http://localhost/payload',
        NULL, NULL, NULL, 'host123', '2023-10-02T10:00:00.000Z',
        '2023-10-02T10:30:00.000Z', '{}', '{}', 'buffered_hash', '2023-10-02T10:45:00.000Z',
        'tag_123', 'v2.0', '1f7b6e8d-cc3c-4b5b-a3f6-5d7b9e2c7f2a', '{}', '15m');