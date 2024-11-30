import 'package:flutter/material.dart';
import '../models/box_model.dart';

class BoxProvider with ChangeNotifier {
  final List<BoxModel> boxes = [
    BoxModel(id: 'XXS', width: 240, height: 240, type: BoxType.xxsmall),
    BoxModel(id: 'XS', width: 100, height: 100, type: BoxType.xsmall),
    BoxModel(id: 'S', width: 240, height: 240, type: BoxType.small),
    BoxModel(id: 'M', width: 240, height: 240, type: BoxType.medium),
    BoxModel(id: 'L', width: 200, height: 240, type: BoxType.large),
    BoxModel(id: 'XL', width: 240, height: 240, type: BoxType.xlarge),
  ];

  void toggleAvailability(String id) {
    final box = boxes.firstWhere((b) => b.id == id);
    box.isAvailable = !box.isAvailable;
    notifyListeners();
  }
}
