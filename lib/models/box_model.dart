class BoxModel {
  final String id;
  final int width;
  final int height;
  final BoxType type;
  bool isAvailable;

  BoxModel({
    required this.id,
    required this.width,
    required this.height,
    required this.type,
    this.isAvailable = true,
  });
}

enum BoxType {
  xxs,
  xs,
  s,
  m,
  l,
  xl,
}
