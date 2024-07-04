import pytest

from testcases import TestCase


def pytest_addoption(parser):
    parser.addoption("--force-aws-upload", action="store_true", default=False,
                     help=("Force AWS upload when building AMI, failing if credentials are not set. "
                           "If not set, the upload will be performed only when credentials are available."))


@pytest.fixture(name="force_aws_upload", scope="session")
def force_aws_upload_fixture(request):
    return request.config.getoption("--force-aws-upload")


# see https://hackebrot.github.io/pytest-tricks/param_id_func/ and
# https://docs.pytest.org/en/7.1.x/reference/reference.html#pytest.hookspec.pytest_make_parametrize_id
def pytest_make_parametrize_id(config, val):
    if isinstance(val, TestCase):
        return f"{val}"
    return None
