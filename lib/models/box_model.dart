enum BoxType { xxsmall, xsmall, small, medium, large, xlarge }

class BoxModel {
  final String id;
  final double width;
  final double height;
  final BoxType type;
  bool isAvailable;

  BoxModel({
    required this.id,
    required this.width,
    required this.height,
    this.type = BoxType.small,
    this.isAvailable = true,
  });
}
