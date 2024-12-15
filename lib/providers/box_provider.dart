import '../models/box_model.dart';
import 'package:flutter/foundation.dart';

class BoxProvider with ChangeNotifier {
  final List<BoxModel> boxes = [];

  BoxProvider() {
    _initializeBoxes();
  }

  void _initializeBoxes() {
    _addBoxes('XXS', 240, 240, BoxType.xxs, 10);
    _addBoxes('XS', 100, 100, BoxType.xs, 10);
    _addBoxes('S', 120, 120, BoxType.s, 10);
    _addBoxes('M', 180, 180, BoxType.m, 10);
    _addBoxes('L', 200, 240, BoxType.l, 10);
    _addBoxes('XL', 300, 300, BoxType.xl, 1);
  }

  void _addBoxes(
      String prefix, int width, int height, BoxType type, int count) {
    for (int i = 1; i <= count; i++) {
      boxes.add(BoxModel(
        id: '$prefix-$i',
        width: width,
        height: height,
        type: type,
        isAvailable: true,
      ));
    }
  }

  void toggleAvailability(String id) {
    final box = boxes.firstWhere((b) => b.id == id);
    box.isAvailable = !box.isAvailable;
    notifyListeners();
  }
}
