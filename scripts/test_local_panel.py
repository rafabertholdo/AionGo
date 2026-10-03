import copy
import unittest

from local_panel import LABEL, reconnect_command


class LocalPanelTests(unittest.TestCase):
    def setUp(self):
        self.database = {"status": "running", "networks": [{"address": "192.168.64.130/24"}]}
        self.panel = {"status": "running", "configuration": {
            "labels": {LABEL: "true"}, "mounts": [], "publishedSockets": [],
            "initProcess": {"environment": ["AION_DB=192.168.64.89", "AION_GS_DB=au_server_gs_java"],
                            "executable": "/panel", "arguments": []},
            "publishedPorts": [{"hostAddress": "127.0.0.1", "hostPort": 8080,
                                "containerPort": 8080, "proto": "tcp"}],
            "image": {"reference": "aiongo-panel-local:0.1.2"}}}

    def test_stale_address_preserves_image_schema_port_and_opt_in(self):
        command = reconnect_command(self.database, self.panel)
        self.assertIn("AION_DB=192.168.64.130", command)
        self.assertNotIn("AION_DB=192.168.64.89", command)
        self.assertIn("AION_GS_DB=au_server_gs_java", command)
        self.assertIn("127.0.0.1:8080:8080/tcp", command)
        self.assertIn(f"{LABEL}=true", command)
        self.assertEqual(command[-1], "aiongo-panel-local:0.1.2")

    def test_current_address_needs_no_restart(self):
        self.panel["configuration"]["initProcess"]["environment"][0] = "AION_DB=192.168.64.130"
        self.assertIsNone(reconnect_command(self.database, self.panel))

    def test_unmanaged_panel_is_untouched(self):
        self.panel["configuration"]["labels"] = {}
        self.assertIsNone(reconnect_command(self.database, self.panel))

    def test_stopped_database_is_untouched(self):
        self.database["status"] = "stopped"
        self.assertIsNone(reconnect_command(self.database, self.panel))

    def test_stopped_panel_is_recreated_even_with_current_address(self):
        self.panel["status"] = "stopped"
        self.panel["configuration"]["initProcess"]["environment"][0] = "AION_DB=192.168.64.130"
        self.assertIsNotNone(reconnect_command(self.database, self.panel))

    def test_custom_mount_is_not_discarded(self):
        self.panel["configuration"]["mounts"] = [{"source": "/custom"}]
        with self.assertRaises(ValueError):
            reconnect_command(self.database, self.panel)

    def test_input_is_not_mutated(self):
        before = copy.deepcopy(self.panel)
        reconnect_command(self.database, self.panel)
        self.assertEqual(before, self.panel)
