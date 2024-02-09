import argparse
import logging
import os
import sys
import yaml

import requirements
import importlib.metadata

base_collections_path = '/usr/share/ansible/collections'
default_file = 'execution-environment.yml'
logger = logging.getLogger(__name__)


def line_is_empty(line):
    return bool((not line.strip()) or line.startswith('#'))


def read_req_file(path):
    """Provide some minimal error and display handling for file reading"""
    if not os.path.exists(path):
        print('Expected requirements file not present at: {0}'.format(os.path.abspath(path)))
    with open(path, 'r') as f:
        return f.read()


def pip_file_data(path):
    pip_content = read_req_file(path)

    pip_lines = []
    for line in pip_content.split('\n'):
        if line_is_empty(line):
            continue
        if line.startswith('-r') or line.startswith('--requirement'):
            _, new_filename = line.split(None, 1)
            new_path = os.path.join(os.path.dirname(path or '.'), new_filename)
            pip_lines.extend(pip_file_data(new_path))
        else:
            pip_lines.append(line)

    return pip_lines


def bindep_file_data(path):
    sys_content = read_req_file(path)

    sys_lines = []
    for line in sys_content.split('\n'):
        if line_is_empty(line):
            continue
        sys_lines.append(line)

    return sys_lines


def process_collection(path):
    """Return a tuple of (python_dependencies, system_dependencies) for the
    collection install path given.
    Both items returned are a list of dependencies.

    :param str path: root directory of collection (this would contain galaxy.yml file)
    """
    CD = CollectionDefinition(path)

    py_file = CD.get_dependency('python')
    pip_lines = []
    if py_file:
        pip_lines = pip_file_data(os.path.join(path, py_file))

    sys_file = CD.get_dependency('system')
    bindep_lines = []
    if sys_file:
        bindep_lines = bindep_file_data(os.path.join(path, sys_file))

    return (pip_lines, bindep_lines)


def process(data_dir=base_collections_path, user_pip=None, user_bindep=None):
    paths = []
    path_root = os.path.join(data_dir, 'ansible_collections')

    # build a list of all the valid collection paths
    if os.path.exists(path_root):
        for namespace in sorted(os.listdir(path_root)):
            if not os.path.isdir(os.path.join(path_root, namespace)):
                continue
            for name in sorted(os.listdir(os.path.join(path_root, namespace))):
                collection_dir = os.path.join(path_root, namespace, name)
                if not os.path.isdir(collection_dir):
                    continue
                files_list = os.listdir(collection_dir)
                if 'galaxy.yml' in files_list or 'MANIFEST.json' in files_list:
                    paths.append(collection_dir)

    # populate the requirements content
    py_req = {}
    sys_req = {}
    for path in paths:
        col_pip_lines, col_sys_lines = process_collection(path)
        CD = CollectionDefinition(path)
        namespace, name = CD.namespace_name()
        key = '{}.{}'.format(namespace, name)

        if col_pip_lines:
            py_req[key] = col_pip_lines

        if col_sys_lines:
            sys_req[key] = col_sys_lines

    # add on entries from user files, if they are given
    if user_pip:
        col_pip_lines = pip_file_data(user_pip)
        if col_pip_lines:
            py_req['user'] = col_pip_lines
    if user_bindep:
        col_sys_lines = bindep_file_data(user_bindep)
        if col_sys_lines:
            sys_req['user'] = col_sys_lines

    return {
        'python': py_req,
        'system': sys_req
    }


def has_content(candidate_file):
    """Beyond checking that the candidate exists, this also assures
    that the file has something other than whitespace,
    which can cause errors when given to pip.
    """
    if not os.path.exists(candidate_file):
        return False
    with open(candidate_file, 'r') as f:
        content = f.read()
    return bool(content.strip().strip('\n'))


class CollectionDefinition:
    """This class represents the dependency metadata for a collection
    should be replaced by logic to hit the Galaxy API if made available
    """

    def __init__(self, collection_path):
        self.reference_path = collection_path
        meta_file = os.path.join(collection_path, 'meta', default_file)
        if os.path.exists(meta_file):
            with open(meta_file, 'r') as f:
                self.raw = yaml.safe_load(f)
        else:
            self.raw = {'version': 1, 'dependencies': {}}
            # Automatically infer requirements for collection
            for entry, filename in [('python', 'requirements.txt'), ('system', 'bindep.txt')]:
                candidate_file = os.path.join(collection_path, filename)
                if has_content(candidate_file):
                    self.raw['dependencies'][entry] = filename

    def target_dir(self):
        namespace, name = self.namespace_name()
        return os.path.join(
            base_collections_path, 'ansible_collections',
            namespace, name
        )

    def namespace_name(self):
        "Returns 2-tuple of namespace and name"
        path_parts = [p for p in self.reference_path.split(os.path.sep) if p]
        return tuple(path_parts[-2:])

    def get_dependency(self, entry):
        """A collection is only allowed to reference a file by a relative path
        which is relative to the collection root
        """
        req_file = self.raw.get('dependencies', {}).get(entry)
        if req_file is None:
            return None
        elif os.path.isabs(req_file):
            raise RuntimeError(
                'Collections must specify relative paths for requirements files. '
                'The file {0} specified by {1} violates this.'.format(
                    req_file, self.reference_path
                )
            )

        return req_file


def simple_combine(reqs):
    """Given a dictionary of requirement lines keyed off collections,
    return a list with the most basic of de-duplication logic,
    and comments indicating the sources based off the collection keys
    """
    consolidated = []
    fancy_lines = []
    for collection, lines in reqs.items():
        for line in lines:
            if line_is_empty(line):
                continue

            base_line = line.split('#')[0].strip()
            if base_line in consolidated:
                i = consolidated.index(base_line)
                fancy_lines[i] += ', {}'.format(collection)
            else:
                fancy_line = base_line + '  # from collection {}'.format(collection)
                consolidated.append(base_line)
                fancy_lines.append(fancy_line)

    return fancy_lines


def parse_args(args=sys.argv[1:]):

    parser = argparse.ArgumentParser(
        prog='introspect',
        description=(
            'ansible-builder introspection; injected and used during execution environment build'
        )
    )

    subparsers = parser.add_subparsers(help='The command to invoke.', dest='action')
    subparsers.required = True

    create_introspect_parser(subparsers)

    args = parser.parse_args(args)

    return args


def run_introspect(args, logger):
    data = process(args.folder, user_pip=args.user_pip, user_bindep=args.user_bindep)
    if args.sanitize:
        logger.info('# Sanitized dependencies for %s', args.folder)
        data_for_write = data
        data['python'] = sanitize_requirements(data['python'])
        data['system'] = simple_combine(data['system'])
    else:
        logger.info('# Dependency data for %s', args.folder)
        data_for_write = data.copy()
        data_for_write['python'] = simple_combine(data['python'])
        data_for_write['system'] = simple_combine(data['system'])

    print('---')
    print(yaml.dump(data, default_flow_style=False))

    if args.write_pip and data.get('python'):
        write_file(args.write_pip, data_for_write.get('python') + [''])
    if args.write_bindep and data.get('system'):
        write_file(args.write_bindep, data_for_write.get('system') + [''])

    sys.exit(0)


def create_introspect_parser(parser):
    introspect_parser = parser.add_parser(
        'introspect',
        help='Introspects collections in folder.',
        description=(
            'Loops over collections in folder and returns data about dependencies. '
            'This is used internally and exposed here for verification. '
            'This is targeted toward collection authors and maintainers.'
        )
    )
    introspect_parser.add_argument('--sanitize', action='store_true',
                                   help=('Sanitize and de-duplicate requirements. '
                                         'This is normally done separately from the introspect script, but this '
                                         'option is given to more accurately test collection content.'))

    introspect_parser.add_argument(
        'folder', default=base_collections_path, nargs='?',
        help=(
            'Ansible collections path(s) to introspect. '
            'This should have a folder named ansible_collections inside of it.'
        )
    )
    # Combine user requirements and collection requirements into single file
    # in the future, could look into passing multilple files to
    # python-builder scripts to be fed multiple files as opposed to this
    introspect_parser.add_argument(
        '--user-pip', dest='user_pip',
        help='An additional file to combine with collection pip requirements.'
    )
    introspect_parser.add_argument(
        '--user-bindep', dest='user_bindep',
        help='An additional file to combine with collection bindep requirements.'
    )
    introspect_parser.add_argument(
        '--write-pip', dest='write_pip',
        help='Write the combined pip requirements file to this location.'
    )
    introspect_parser.add_argument(
        '--write-bindep', dest='write_bindep',
        help='Write the combined bindep requirements file to this location.'
    )

    return introspect_parser


EXCLUDE_REQUIREMENTS = frozenset((
    # obviously already satisfied or unwanted
    'ansible', 'ansible-base', 'python', 'ansible-core',
    # general python test requirements
    'tox', 'pycodestyle', 'yamllint', 'pylint',
    'flake8', 'pytest', 'pytest-xdist', 'coverage', 'mock', 'testinfra',
    # test requirements highly specific to Ansible testing
    'ansible-lint', 'molecule', 'galaxy-importer', 'voluptuous',
    # already present in image for py3 environments
    'yaml', 'pyyaml', 'json',
))


def sanitize_requirements(collection_py_reqs):
    """
    Cleanup Python requirements by removing duplicates and excluded packages.

    The user requirements file will go through the deduplication process, but
    skips the special package exclusion process.

    :param dict collection_py_reqs: A dict of lists of Python requirements, keyed
        by fully qualified collection name. The special key `user` holds requirements
        from the user specified requirements file from the ``--user-pip`` CLI option.

    :returns: A finalized list of sanitized Python requirements.
    """
    # de-duplication
    consolidated = []
    seen_pkgs = set()

    for collection, lines in collection_py_reqs.items():
        try:
            for req in requirements.parse('\n'.join(lines)):
                if req.specifier:
                    req.name = importlib.metadata.Prepared(req.name).normalized
                req.collections = [collection]  # add backref for later
                if req.name is None:
                    consolidated.append(req)
                    continue
                if req.name in seen_pkgs:
                    for prior_req in consolidated:
                        if req.name == prior_req.name:
                            prior_req.specs.extend(req.specs)
                            prior_req.collections.append(collection)
                            break
                    continue
                consolidated.append(req)
                seen_pkgs.add(req.name)
        except Exception as e:
            logger.warning('Warning: failed to parse requirements from %s, error: %s', collection, e)

    # removal of unwanted packages
    sanitized = []
    for req in consolidated:
        # Exclude packages, unless it was present in the user supplied requirements.
        if req.name and req.name.lower() in EXCLUDE_REQUIREMENTS and 'user' not in req.collections:
            logger.debug('# Excluding requirement %s from %s', req.name, req.collections)
            continue
        if req.vcs or req.uri:
            # Requirement like git+ or http return as-is
            new_line = req.line
        elif req.name:
            specs = ['{0}{1}'.format(cmp, ver) for cmp, ver in req.specs]
            new_line = req.name + ','.join(specs)
        else:
            raise RuntimeError('Could not process {0}'.format(req.line))

        sanitized.append(new_line + '  # from collection {}'.format(','.join(req.collections)))

    return sanitized


def write_file(filename: str, lines: list) -> bool:
    parent_dir = os.path.dirname(filename)
    if parent_dir and not os.path.exists(parent_dir):
        logger.warning('Creating parent directory for %s', filename)
        os.makedirs(parent_dir)
    new_text = '\n'.join(lines)
    if os.path.exists(filename):
        with open(filename, 'r') as f:
            if f.read() == new_text:
                logger.debug("File %s is already up-to-date.", filename)
                return False
            else:
                logger.warning('File %s had modifications and will be rewritten', filename)
    with open(filename, 'w') as f:
        f.write(new_text)
    return True


def main():
    args = parse_args()

    if args.action == 'introspect':
        run_introspect(args, logger)

    logger.error("An error has occurred.")
    sys.exit(1)


if __name__ == '__main__':
    main()
