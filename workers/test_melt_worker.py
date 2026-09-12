import unittest

from melt_worker import container_for


class ContainerForTest(unittest.TestCase):
    def test_atomic_temp_suffix_preserves_real_container(self) -> None:
        self.assertEqual(container_for("render.mp4.tmp"), "mp4")
        self.assertEqual(container_for("render.mov.tmp"), "mov")
        self.assertEqual(container_for("render.mkv.tmp"), "matroska")
        self.assertEqual(container_for("render.webm.tmp"), "webm")

    def test_unknown_or_missing_extension_defaults_to_mp4(self) -> None:
        self.assertEqual(container_for("render.unknown.tmp"), "mp4")
        self.assertEqual(container_for("render.tmp"), "mp4")


if __name__ == "__main__":
    unittest.main()
