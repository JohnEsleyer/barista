// Package classfile decodes JVM class files into an in-memory representation.
//
// The package owns Stage 1 of the Barista pipeline: validating the class-file
// magic and version, parsing the constant pool, and extracting class metadata,
// fields, methods, and bytecode-bearing attributes.
package classfile
